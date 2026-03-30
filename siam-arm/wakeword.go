package main

// wakeword.go — Phase 5.2: Wake Word Detection (Vosk)
//
// Runs an always-on mic loop using a small Vosk speech recognition model.
// Detects configurable wake words ("kaidos", "kook", "auric") and notifies
// master via gRPC so the agent can enter active listening mode.
//
// Vosk: https://alphacephei.com/vosk/
// Model: vosk-model-small-th-0.13 (~80MB Thai) or vosk-model-small-en-us-0.15 (~40MB EN)
//
// Architecture:
//   mic (ALSA/termux) → 16kHz mono WAV chunks → vosk_sidecar.py → match keywords → gRPC notify

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ─── Wake Word Config ─────────────────────────────────────────────────────────

// defaultWakeWords are the keywords each agent responds to.
// Overridable via WAKE_WORDS env var (comma-separated).
var defaultWakeWords = []string{"kaidos", "kook", "auric", "ไกออส", "คุก"}

func getWakeWords() []string {
	if env := strings.TrimSpace(os.Getenv("WAKE_WORDS")); env != "" {
		words := strings.Split(env, ",")
		var out []string
		for _, w := range words {
			if w = strings.TrimSpace(strings.ToLower(w)); w != "" {
				out = append(out, w)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return defaultWakeWords
}

// voskModelDir returns the directory where Vosk models are stored.
func voskModelDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".picoclaw", "vosk")
}

// bestVoskModel returns the path to the best available Vosk model.
func bestVoskModel() string {
	dir := voskModelDir()
	// Prefer Thai, then English
	candidates := []string{
		filepath.Join(dir, "vosk-model-small-th-0.13"),
		filepath.Join(dir, "vosk-model-small-en-us-0.15"),
		filepath.Join(dir, "vosk-model-en-us-0.22-lgraph"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// ─── Wake Word Loop State ─────────────────────────────────────────────────────

var (
	wakeWordActive atomic.Bool
	wakeWordMu     sync.Mutex
	wakeWordCancel context.CancelFunc
)

// IsWakeWordRunning returns true if the wake word loop is active.
func IsWakeWordRunning() bool {
	return wakeWordActive.Load()
}

// StartWakeWordLoop starts the continuous wake word detection loop.
// If already running, this is a no-op.
// onWake is called each time a wake word is detected with the matched word.
func StartWakeWordLoop(ctx context.Context, onWake func(word string)) error {
	wakeWordMu.Lock()
	defer wakeWordMu.Unlock()

	if wakeWordActive.Load() {
		return nil // already running
	}

	model := bestVoskModel()
	if model == "" {
		return fmt.Errorf("no Vosk model found in %s — run EnsureVoskInstalled() first", voskModelDir())
	}

	loopCtx, cancel := context.WithCancel(ctx)
	wakeWordCancel = cancel
	wakeWordActive.Store(true)

	go runWakeWordLoop(loopCtx, model, onWake)
	log.Printf("🎤 [WakeWord] Started — model: %s | words: %v", filepath.Base(model), getWakeWords())
	return nil
}

// StopWakeWordLoop stops the wake word detection loop.
func StopWakeWordLoop() {
	wakeWordMu.Lock()
	defer wakeWordMu.Unlock()
	if wakeWordCancel != nil {
		wakeWordCancel()
	}
	wakeWordActive.Store(false)
	log.Println("🛑 [WakeWord] Stopped")
}

// runWakeWordLoop runs continuously: record 2s chunk → Vosk STT → check keywords.
func runWakeWordLoop(ctx context.Context, model string, onWake func(word string)) {
	defer func() {
		wakeWordActive.Store(false)
		log.Println("🛑 [WakeWord] loop exited")
	}()

	words := getWakeWords()

	for {
		if ctx.Err() != nil {
			return
		}
		// Don't detect while TTS is speaking (echo prevention)
		if isSpeaking.Load() {
			time.Sleep(300 * time.Millisecond)
			continue
		}

		wavPath := fmt.Sprintf("/tmp/wake-%d.wav", time.Now().UnixNano())
		// Record 2-second chunk at 16kHz mono
		var recErr error
		if isTermux() {
			recErr = exec.CommandContext(ctx, "termux-microphone-record",
				"-e", "WAV", "-r", "16000", "-c", "1", "-l", "2", "-f", wavPath).Run()
		} else {
			recErr = recordWAVParams(ctx, wavPath, "2", 16000, 1)
		}
		if recErr != nil {
			os.Remove(wavPath)
			if ctx.Err() != nil {
				return
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Run Vosk STT on the chunk
		transcript := voskTranscribe(ctx, model, wavPath)
		os.Remove(wavPath)

		if transcript == "" {
			continue
		}
		transcriptLower := strings.ToLower(transcript)

		for _, word := range words {
			if strings.Contains(transcriptLower, word) {
				log.Printf("🔔 [WakeWord] Detected: %q in %q", word, transcript)
				onWake(word)
				// Brief pause after detection to avoid re-triggering
				time.Sleep(2 * time.Second)
				break
			}
		}
	}
}

// voskTranscribe runs Vosk STT on a WAV file and returns the transcript.
func voskTranscribe(ctx context.Context, model, wavPath string) string {
	pyScript := `import sys, json, wave
from vosk import Model, KaldiRecognizer
model_path, wav_path = sys.argv[1], sys.argv[2]
try:
    model = Model(model_path)
    wf = wave.open(wav_path, 'rb')
    rec = KaldiRecognizer(model, wf.getframerate())
    rec.SetWords(True)
    results = []
    while True:
        data = wf.readframes(4000)
        if not data:
            break
        if rec.AcceptWaveform(data):
            r = json.loads(rec.Result())
            if r.get('text'):
                results.append(r['text'])
    r = json.loads(rec.FinalResult())
    if r.get('text'):
        results.append(r['text'])
    print(' '.join(results))
except Exception as e:
    print('', end='')
`
	out, err := exec.CommandContext(ctx, "python3", "-c", pyScript, model, wavPath).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// recordWAVParams records audio with custom sample rate and channels via arecord/sox/ffmpeg.
func recordWAVParams(ctx context.Context, outputPath, duration string, rate, channels int) error {
	rateStr := fmt.Sprintf("%d", rate)
	chanStr := fmt.Sprintf("%d", channels)
	durSecs := duration

	// Try arecord (ALSA)
	if _, err := exec.LookPath("arecord"); err == nil {
		return exec.CommandContext(ctx, "arecord",
			"-f", "S16_LE", "-r", rateStr, "-c", chanStr,
			"-d", durSecs, outputPath).Run()
	}
	// Try sox
	if _, err := exec.LookPath("sox"); err == nil {
		return exec.CommandContext(ctx, "sox",
			"-d", "-r", rateStr, "-c", chanStr,
			outputPath, "trim", "0", durSecs).Run()
	}
	// Try ffmpeg
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		return exec.CommandContext(ctx, "ffmpeg",
			"-y", "-f", "alsa", "-i", "default",
			"-t", durSecs, "-ar", rateStr, "-ac", chanStr,
			outputPath).Run()
	}
	return fmt.Errorf("no audio recording tool available (arecord/sox/ffmpeg)")
}

// ─── Vosk Installation ────────────────────────────────────────────────────────

// EnsureVoskInstalled checks that vosk Python package and a model are available.
// Downloads the smallest model if missing. Runs once in background at startup.
func EnsureVoskInstalled(ctx context.Context) {
	// Check vosk Python package
	checkOut, _ := exec.CommandContext(ctx, "python3", "-c", "import vosk; print('ok')").Output()
	if !strings.Contains(string(checkOut), "ok") {
		log.Println("📥 [WakeWord] Installing vosk Python package...")
		pip := "pip3"
		if isTermux() {
			pip = "pip"
		}
		if err := exec.CommandContext(ctx, pip, "install", "vosk").Run(); err != nil {
			log.Printf("⚠️ [WakeWord] vosk install failed: %v (wake word will be unavailable)", err)
			return
		}
	}

	if model := bestVoskModel(); model != "" {
		log.Printf("✅ [WakeWord] Vosk model already present: %s", filepath.Base(model))
		return
	}

	// Download smallest available model
	modelDir := voskModelDir()
	os.MkdirAll(modelDir, 0755)

	var modelURL, modelName string
	if isTermux() {
		// Android: prefer Thai small model (~80MB)
		modelName = "vosk-model-small-th-0.13"
		modelURL = "https://alphacephei.com/vosk/models/vosk-model-small-th-0.13.zip"
	} else {
		// Linux: English small model (~40MB, faster for keyword spotting)
		modelName = "vosk-model-small-en-us-0.15"
		modelURL = "https://alphacephei.com/vosk/models/vosk-model-small-en-us-0.15.zip"
	}

	zipPath := filepath.Join(modelDir, modelName+".zip")
	log.Printf("📥 [WakeWord] Downloading Vosk model: %s", modelName)
	if err := downloadFile(ctx, modelURL, zipPath); err != nil {
		log.Printf("⚠️ [WakeWord] Model download failed: %v", err)
		return
	}
	defer os.Remove(zipPath)

	if err := exec.CommandContext(ctx, "unzip", "-q", zipPath, "-d", modelDir).Run(); err != nil {
		log.Printf("⚠️ [WakeWord] Model unzip failed: %v", err)
		return
	}
	log.Printf("✅ [WakeWord] Vosk model ready: %s", modelName)
}
