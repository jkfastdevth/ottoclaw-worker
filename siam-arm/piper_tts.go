package main

// piper_tts.go — Phase 5.1: Piper TTS Integration
//
// Piper is a fast, local neural TTS engine that runs entirely on CPU.
// Much higher quality than espeak-ng. Single binary + ONNX voice models.
//
// Piper binary: github.com/rhasspy/piper/releases
// Thai model:   rhasspy/piper-voices (th_TH-tacotron_ddc-medium)
// EN model:     en_US-lessac-medium
//
// Usage: echo "text" | piper --model /path/to/model.onnx --output_file out.wav

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ─── Piper Paths ─────────────────────────────────────────────────────────────

// piperDir returns the directory where Piper binary and models are stored.
func piperDir() string {
	home, _ := os.UserHomeDir()
	if isTermux() {
		return filepath.Join(home, ".picoclaw", "piper")
	}
	return filepath.Join(home, ".picoclaw", "piper")
}

// piperBinaryPath returns the absolute path to the piper binary.
func piperBinaryPath() string {
	return filepath.Join(piperDir(), "piper")
}

// piperModelPath returns the ONNX model path for the given language.
// lang: "th" → Thai, anything else → English.
func piperModelPath(lang string) string {
	dir := filepath.Join(piperDir(), "models")
	if strings.HasPrefix(lang, "th") {
		return filepath.Join(dir, "th_TH-tacotron_ddc-medium.onnx")
	}
	return filepath.Join(dir, "en_US-lessac-medium.onnx")
}

// IsPiperAvailable returns true if the piper binary and at least one model exist.
func IsPiperAvailable() bool {
	if _, err := os.Stat(piperBinaryPath()); err != nil {
		// Also check system PATH
		if _, pathErr := exec.LookPath("piper"); pathErr != nil {
			return false
		}
	}
	// Check at least EN model
	if _, err := os.Stat(piperModelPath("en")); err != nil {
		if _, err2 := os.Stat(piperModelPath("th")); err2 != nil {
			return false
		}
	}
	return true
}

// piperExec returns the piper binary to use (local install first, then PATH).
func piperExec() string {
	local := piperBinaryPath()
	if _, err := os.Stat(local); err == nil {
		return local
	}
	if path, err := exec.LookPath("piper"); err == nil {
		return path
	}
	return "piper"
}

// ─── TTS via Piper ────────────────────────────────────────────────────────────

// SpeakWithPiper synthesizes text to a WAV file using Piper TTS.
// Returns the WAV path on success. Caller is responsible for removing the file.
func SpeakWithPiper(ctx context.Context, text, lang, outputPath string) error {
	if outputPath == "" {
		outputPath = fmt.Sprintf("/tmp/piper-%d.wav", uniqueNano())
	}
	model := piperModelPath(lang)
	if _, err := os.Stat(model); err != nil {
		// Fallback: try other model
		if strings.HasPrefix(lang, "th") {
			model = piperModelPath("en")
		} else {
			model = piperModelPath("th")
		}
		if _, err2 := os.Stat(model); err2 != nil {
			return fmt.Errorf("piper model not found (lang=%s): %w", lang, err2)
		}
	}

	binary := piperExec()
	// Piper reads text from stdin
	cmd := exec.CommandContext(ctx, binary,
		"--model", model,
		"--output_file", outputPath,
	)
	cmd.Stdin = strings.NewReader(text)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("piper failed: %w — %s", err, string(out))
	}
	return nil
}

// SpeakWithPiperAndPlay synthesizes text and plays it immediately.
// Returns true if speech was played successfully.
func SpeakWithPiperAndPlay(ctx context.Context, text, lang string) bool {
	wavPath := fmt.Sprintf("/tmp/piper-play-%d.wav", uniqueNano())
	defer os.Remove(wavPath)

	if err := SpeakWithPiper(ctx, text, lang, wavPath); err != nil {
		log.Printf("⚠️ [PiperTTS] synth failed: %v", err)
		return false
	}

	// Try audio players in order
	players := [][]string{
		{"aplay", wavPath},
		{"ffplay", "-nodisp", "-autoexit", wavPath},
		{"mpv", "--no-terminal", wavPath},
		{"paplay", wavPath},
	}
	if isTermux() {
		// Termux: ffplay or termux-media-player
		players = [][]string{
			{"ffplay", "-nodisp", "-autoexit", wavPath},
			{"termux-media-player", "play", wavPath},
		}
	}

	for _, args := range players {
		if exec.CommandContext(ctx, args[0], args[1:]...).Run() == nil {
			return true
		}
	}
	log.Printf("⚠️ [PiperTTS] no audio player succeeded for %s", wavPath)
	return false
}

// ─── Auto-Download Piper ──────────────────────────────────────────────────────

// EnsurePiperInstalled downloads and installs Piper if not already present.
// Runs once in background at startup.
func EnsurePiperInstalled(ctx context.Context) {
	if IsPiperAvailable() {
		log.Println("✅ [PiperTTS] Already installed")
		return
	}
	log.Println("📥 [PiperTTS] Installing Piper TTS...")

	dir := piperDir()
	os.MkdirAll(filepath.Join(dir, "models"), 0755)

	// Determine platform archive
	arch := runtime.GOARCH
	goos := runtime.GOOS
	if isTermux() {
		goos = "linux"
	}

	var archiveName string
	switch {
	case goos == "linux" && arch == "amd64":
		archiveName = "piper_linux_x86_64.tar.gz"
	case goos == "linux" && arch == "arm64":
		archiveName = "piper_linux_aarch64.tar.gz"
	case goos == "linux" && arch == "arm":
		archiveName = "piper_linux_armv7l.tar.gz"
	case goos == "darwin" && arch == "amd64":
		archiveName = "piper_macos_x64.tar.gz"
	case goos == "darwin" && arch == "arm64":
		archiveName = "piper_macos_aarch64.tar.gz"
	default:
		log.Printf("⚠️ [PiperTTS] Unsupported platform: %s/%s", goos, arch)
		return
	}

	baseURL := "https://github.com/rhasspy/piper/releases/latest/download/"
	archivePath := filepath.Join(dir, archiveName)

	if err := downloadFile(ctx, baseURL+archiveName, archivePath); err != nil {
		log.Printf("⚠️ [PiperTTS] Download failed: %v", err)
		return
	}
	defer os.Remove(archivePath)

	// Extract
	if err := exec.CommandContext(ctx, "tar", "-xzf", archivePath, "-C", dir).Run(); err != nil {
		log.Printf("⚠️ [PiperTTS] Extract failed: %v", err)
		return
	}

	// Piper extracts to piper/ subdirectory — move binary up
	extractedBin := filepath.Join(dir, "piper", "piper")
	if _, err := os.Stat(extractedBin); err == nil {
		os.Rename(extractedBin, piperBinaryPath())
		os.Chmod(piperBinaryPath(), 0755)
		os.RemoveAll(filepath.Join(dir, "piper"))
	}

	// Download voice models
	downloadPiperModels(ctx, dir)

	if IsPiperAvailable() {
		log.Println("✅ [PiperTTS] Installation complete")
	} else {
		log.Println("⚠️ [PiperTTS] Installation may be incomplete — check logs")
	}
}

// downloadPiperModels fetches ONNX voice models for Thai and English.
func downloadPiperModels(ctx context.Context, dir string) {
	type model struct {
		name    string
		onnxURL string
		jsonURL string
	}
	modelsDir := filepath.Join(dir, "models")
	models := []model{
		{
			name:    "en_US-lessac-medium",
			onnxURL: "https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/lessac/medium/en_US-lessac-medium.onnx",
			jsonURL: "https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/lessac/medium/en_US-lessac-medium.onnx.json",
		},
		{
			name:    "th_TH-tacotron_ddc-medium",
			onnxURL: "https://huggingface.co/rhasspy/piper-voices/resolve/main/th/th_TH/tacotron_ddc/medium/th_TH-tacotron_ddc-medium.onnx",
			jsonURL: "https://huggingface.co/rhasspy/piper-voices/resolve/main/th/th_TH/tacotron_ddc/medium/th_TH-tacotron_ddc-medium.onnx.json",
		},
	}

	for _, m := range models {
		onnxPath := filepath.Join(modelsDir, m.name+".onnx")
		jsonPath := filepath.Join(modelsDir, m.name+".onnx.json")
		if _, err := os.Stat(onnxPath); err == nil {
			log.Printf("✅ [PiperTTS] Model already present: %s", m.name)
			continue
		}
		log.Printf("📥 [PiperTTS] Downloading model: %s", m.name)
		if err := downloadFile(ctx, m.onnxURL, onnxPath); err != nil {
			log.Printf("⚠️ [PiperTTS] Model download failed (%s): %v", m.name, err)
			continue
		}
		downloadFile(ctx, m.jsonURL, jsonPath) //nolint:errcheck
		log.Printf("✅ [PiperTTS] Model ready: %s", m.name)
	}
}

// ─── HTTP Download Helper ─────────────────────────────────────────────────────

func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

// uniqueNano returns a unique nanosecond timestamp for temp filenames.
// Avoids import of sync/atomic just for this.
var uniqueNanoCounter int64

func uniqueNano() int64 {
	return uniqueNanoCounter + int64(os.Getpid())
}
