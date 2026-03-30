package main

// vision.go — Phase C: Multi-modal Vision (LLaVA / MiniCPM-V on Ollama)
//
// Allows the Control Brain to "see" by sending images to a local vision model.
// Works offline — no cloud API required.
//
// Supported models (auto-selected by hardware):
//   Android/Termux: moondream (400MB, fast)  → llava:7b fallback
//   Linux desktop:  llava:7b (4.7GB)          → minicpm-v:8b fallback
//   Linux light:    moondream                  → llava-phi3 (2.9GB)
//
// Ollama vision API: POST /api/chat with images: ["<base64>"] in messages.

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// ─── Vision Model Selection ───────────────────────────────────────────────────

// visionModel returns the best local vision model for this hardware.
func visionModel() string {
	if m := strings.TrimSpace(os.Getenv("VISION_MODEL")); m != "" {
		return m
	}
	if isTermux() {
		return "moondream" // ~400MB — runs on Android
	}
	return "llava:7b" // ~4.7GB — good for Linux (CPU)
}

// visionModelFallbacks returns fallback models in order if primary unavailable.
func visionModelFallbacks() []string {
	if isTermux() {
		return []string{"moondream", "llava-phi3", "llava:7b"}
	}
	return []string{"llava:7b", "llava-phi3", "moondream", "minicpm-v"}
}

// ─── Vision Inference ─────────────────────────────────────────────────────────

// VisionResponse holds the result of a vision query.
type VisionResponse struct {
	Content   string `json:"content"`
	Model     string `json:"model"`
	LatencyMs int64  `json:"latency_ms"`
}

// QueryVisionBrain sends an image + text prompt to the local vision model.
// imagePath: absolute path to a JPEG/PNG file.
// prompt: what to ask about the image (e.g. "Describe this image", "Read the text").
func QueryVisionBrain(ctx context.Context, imagePath, prompt string) (*VisionResponse, error) {
	// Read and base64-encode the image
	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("read image: %w", err)
	}
	imgB64 := base64.StdEncoding.EncodeToString(imgData)

	host := ollamaHost()
	start := time.Now()

	// Try models in order until one succeeds
	for _, model := range visionModelFallbacks() {
		resp, err := callOllamaVision(ctx, host, model, prompt, imgB64)
		if err == nil {
			resp.LatencyMs = time.Since(start).Milliseconds()
			log.Printf("👁️ [Vision] model=%s latency=%dms", model, resp.LatencyMs)
			return resp, nil
		}
		log.Printf("⚠️ [Vision] model %s failed: %v — trying next", model, err)
	}
	return nil, fmt.Errorf("all vision models failed")
}

// callOllamaVision calls Ollama /api/chat with the vision model and image.
func callOllamaVision(ctx context.Context, host, model, prompt, imgB64 string) (*VisionResponse, error) {
	payload := map[string]interface{}{
		"model": model,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": prompt,
				"images":  []string{imgB64},
			},
		},
		"stream":  false,
		"options": map[string]interface{}{"temperature": 0.3, "num_ctx": 2048},
	}
	jsonData, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, "POST", host+"/api/chat", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ollama HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return &VisionResponse{Content: result.Message.Content, Model: model}, nil
}

// ─── Vision Hardware Tool ─────────────────────────────────────────────────────

// AnalyzePhoto takes a photo and returns vision model analysis.
// If imagePath is empty, captures a fresh photo from the camera.
func AnalyzePhoto(ctx context.Context, imagePath, prompt string) (string, error) {
	// Capture if no path provided
	if imagePath == "" {
		var err error
		imagePath, err = TakePhoto(ctx, fmt.Sprintf("/tmp/vision-%d.jpg", time.Now().UnixNano()))
		if err != nil {
			return "", fmt.Errorf("capture failed: %w", err)
		}
		defer os.Remove(imagePath)
	}

	if prompt == "" {
		prompt = "Describe what you see in this image in detail. If there is text, read it."
	}

	if err := ensureOllamaRunning(ctx); err != nil {
		return "", fmt.Errorf("ollama not running: %w", err)
	}

	resp, err := QueryVisionBrain(ctx, imagePath, prompt)
	if err != nil {
		return "", err
	}

	out, _ := json.Marshal(map[string]interface{}{
		"description": resp.Content,
		"model":       resp.Model,
		"latency_ms":  resp.LatencyMs,
	})
	return string(out), nil
}

// ─── Vision Model Setup ───────────────────────────────────────────────────────

// EnsureVisionModelInstalled pulls the vision model if not present.
func EnsureVisionModelInstalled(ctx context.Context) {
	if err := ensureOllamaRunning(ctx); err != nil {
		log.Printf("⚠️ [Vision] Ollama unavailable: %v", err)
		return
	}
	model := visionModel()
	if err := pullModelIfNeeded(ctx, model); err != nil {
		log.Printf("⚠️ [Vision] Model pull failed (%s): %v", model, err)
		return
	}
	log.Printf("✅ [Vision] Ready — model: %s", model)
}

// ─── SYSTEM_VISION handler ────────────────────────────────────────────────────

// HandleVisionCommand processes a SYSTEM_VISION command from master.
// Payload JSON: {"image_b64":"...","image_path":"...","prompt":"..."}
// image_b64 or image_path must be set. image_b64 takes priority.
func HandleVisionCommand(ctx context.Context, payload string) string {
	var req struct {
		ImageB64  string `json:"image_b64"`   // base64-encoded image
		ImagePath string `json:"image_path"`  // local file path
		Prompt    string `json:"prompt"`
		CaptureNow bool  `json:"capture_now"` // take a fresh photo
	}
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		// Treat bare string as prompt with capture
		req.Prompt = payload
		req.CaptureNow = true
	}

	if req.Prompt == "" {
		req.Prompt = "Describe what you see in detail. Read any visible text."
	}

	var imagePath string
	var cleanup bool

	if req.ImageB64 != "" {
		// Decode base64 image to temp file
		imgData, err := base64.StdEncoding.DecodeString(req.ImageB64)
		if err != nil {
			return fmt.Sprintf(`{"error":"invalid image_b64: %s"}`, err.Error())
		}
		imagePath = fmt.Sprintf("/tmp/vision-recv-%d.jpg", time.Now().UnixNano())
		if err := os.WriteFile(imagePath, imgData, 0600); err != nil {
			return fmt.Sprintf(`{"error":"write image: %s"}`, err.Error())
		}
		cleanup = true
	} else if req.ImagePath != "" {
		imagePath = req.ImagePath
	} else {
		// Capture fresh photo
		req.CaptureNow = true
	}

	if req.CaptureNow && imagePath == "" {
		var err error
		imagePath, err = TakePhoto(ctx, fmt.Sprintf("/tmp/vision-capture-%d.jpg", time.Now().UnixNano()))
		if err != nil {
			return fmt.Sprintf(`{"error":"capture failed: %s"}`, err.Error())
		}
		cleanup = true
	}
	if cleanup {
		defer os.Remove(imagePath)
	}

	if err := ensureOllamaRunning(ctx); err != nil {
		return fmt.Sprintf(`{"error":"ollama: %s"}`, err.Error())
	}

	resp, err := QueryVisionBrain(ctx, imagePath, req.Prompt)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}

	out, _ := json.Marshal(map[string]interface{}{
		"description": resp.Content,
		"model":       resp.Model,
		"latency_ms":  resp.LatencyMs,
		"prompt":      req.Prompt,
	})
	return string(out)
}
