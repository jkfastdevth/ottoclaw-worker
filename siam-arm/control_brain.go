package main

// control_brain.go — Phase 4B: Dual-Brain Architecture
//
// Manages a local Ollama "Control Brain" that runs alongside the main
// ottoclaw brain.  The Control Brain handles simple tasks locally
// (low-latency, offline-capable) while the Thinking Brain (master LLM)
// handles complex reasoning via gRPC.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ─── Model Selection ─────────────────────────────────────────────────────────

// agentSpecializedModel maps agent soul names to their specialist Ollama model.
// Each agent identity gets a model tuned for its role.
var agentSpecializedModel = map[string]string{
	"kaidos": "deepseek-coder:1.3b", // coding specialist (~900MB)
	"kook":   "qwen2.5:0.5b",        // conversation specialist (~800MB)
	"auric":  "phi3.5-mini",          // reasoning/planning (~2.2GB)
}

// controlBrainModel returns the best local model for this agent and hardware.
// Priority: CONTROL_BRAIN_MODEL env > agent soul specialization > hardware default.
func controlBrainModel() string {
	if m := strings.TrimSpace(os.Getenv("CONTROL_BRAIN_MODEL")); m != "" {
		return m
	}
	// Specialization by agent soul identity
	soul := strings.ToLower(strings.TrimSpace(os.Getenv("SOUL_ID")))
	if soul == "" {
		soul = strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_ID")))
	}
	if soul != "" {
		// Strip suffixes like "kaidos-v2" → "kaidos"
		for name, model := range agentSpecializedModel {
			if strings.HasPrefix(soul, name) {
				if isTermux() && model == "phi3.5-mini" {
					// phi3.5 too large for Android — fall back to qwen2.5:0.5b
					return "qwen2.5:0.5b"
				}
				return model
			}
		}
	}
	if isTermux() {
		return "qwen2.5:0.5b" // ~800MB RAM — runs on Android
	}
	return "qwen2.5:3b" // ~2.2GB RAM — good for Linux/desktop
}

// ollamaHost returns the Ollama server URL.
func ollamaHost() string {
	if h := strings.TrimSpace(os.Getenv("OLLAMA_HOST")); h != "" {
		return h
	}
	return "http://localhost:11434"
}

// ─── Ollama Process Management ───────────────────────────────────────────────

var (
	ollamaProc   *exec.Cmd
	ollamaProcMu sync.Mutex
	ollamaReady  atomic.Bool
)

// ensureOllamaRunning starts the Ollama server if not already running.
func ensureOllamaRunning(ctx context.Context) error {
	// Check if already responding
	if ollamaReady.Load() {
		return nil
	}
	host := ollamaHost()
	if isOllamaAlive(host) {
		ollamaReady.Store(true)
		return nil
	}

	// Find ollama binary
	ollamaPath, err := exec.LookPath("ollama")
	if err != nil {
		return fmt.Errorf("ollama not found in PATH — install with: curl -fsSL https://ollama.com/install.sh | sh")
	}

	ollamaProcMu.Lock()
	defer ollamaProcMu.Unlock()

	log.Println("🧠 [ControlBrain] Starting Ollama server...")
	cmd := exec.CommandContext(ctx, ollamaPath, "serve")
	cmd.Env = append(os.Environ(), "OLLAMA_ORIGINS=*")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ollama: %w", err)
	}
	ollamaProc = cmd

	// Wait up to 30s for Ollama to be ready
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if isOllamaAlive(host) {
			ollamaReady.Store(true)
			log.Println("🧠 [ControlBrain] Ollama ready")
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("ollama did not respond within 30s")
}

// isOllamaAlive pings the Ollama /api/tags endpoint.
func isOllamaAlive(host string) bool {
	c := &http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get(host + "/api/tags")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

// pullModelIfNeeded pulls the model if not already present locally.
func pullModelIfNeeded(ctx context.Context, model string) error {
	host := ollamaHost()
	// Check if model exists
	c := &http.Client{Timeout: 5 * time.Second}
	resp, err := c.Get(host + "/api/tags")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), `"`+model+`"`) {
		return nil // already present
	}

	log.Printf("🧠 [ControlBrain] Pulling model %s (first-time setup)...", model)
	payload, _ := json.Marshal(map[string]interface{}{"name": model, "stream": false})
	pullReq, _ := http.NewRequestWithContext(ctx, "POST", host+"/api/pull", bytes.NewBuffer(payload))
	pullReq.Header.Set("Content-Type", "application/json")
	pullResp, err := (&http.Client{Timeout: 30 * time.Minute}).Do(pullReq)
	if err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}
	defer pullResp.Body.Close()
	io.Copy(io.Discard, pullResp.Body)
	if pullResp.StatusCode != 200 {
		return fmt.Errorf("pull returned HTTP %d", pullResp.StatusCode)
	}
	log.Printf("🧠 [ControlBrain] Model %s ready", model)
	return nil
}

// ─── Brain Router ────────────────────────────────────────────────────────────

// BrainTarget indicates which brain should handle the request.
type BrainTarget int

const (
	BrainLocal  BrainTarget = iota // Control Brain (local Ollama)
	BrainMaster                    // Thinking Brain (master LLM via gRPC)
)

// classifyTask decides whether a task should use the master LLM or local brain.
// Policy: Master LLM is ALWAYS preferred when reachable. Local Ollama is only
// used as offline fallback. This prevents local Ollama cold-starts from blocking
// mission completion on Linux nodes (kaidos, kook) where Ollama may be slow.
func classifyTask(prompt string, masterReachable bool) BrainTarget {
	// Offline: always local — no other choice
	if !masterReachable {
		return BrainLocal
	}

	// Master is reachable → always use master LLM for reliability and speed.
	// Local brain is reserved for offline/fallback scenarios only.
	// This ensures kaidos/kook on Linux don't block on slow local Ollama.
	_ = prompt // prompt reserved for future fine-grained routing if needed
	return BrainMaster
}

// ─── Control Brain Inference ─────────────────────────────────────────────────

// ControlBrainResponse is the response from the local Control Brain.
type ControlBrainResponse struct {
	Content   string `json:"content"`
	Model     string `json:"model"`
	LatencyMs int64  `json:"latency_ms"`
	Local     bool   `json:"local"`
}

// QueryControlBrain sends a prompt to the local Ollama model and returns the reply.
// systemPrompt may be empty (uses a default assistant prompt).
func QueryControlBrain(ctx context.Context, systemPrompt, userPrompt string) (*ControlBrainResponse, error) {
	host := ollamaHost()
	model := controlBrainModel()

	if systemPrompt == "" {
		agentID := strings.TrimSpace(os.Getenv("AGENT_ID"))
		if agentID == "" {
			agentID = "worker"
		}
		systemPrompt = fmt.Sprintf(
			"You are the Control Brain of agent '%s' running on a local device. "+
				"Handle simple, fast, or offline tasks. "+
				"For complex tasks reply ONLY with: [[ESCALATE]] and nothing else. "+
				"Always respond in the same language as the user.",
			agentID,
		)
	}

	msgs := []map[string]interface{}{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": userPrompt},
	}
	payload := map[string]interface{}{
		"model":    model,
		"messages": msgs,
		"stream":   false,
		"options":  map[string]interface{}{"temperature": 0.5, "num_ctx": 2048},
	}
	jsonData, _ := json.Marshal(payload)

	start := time.Now()
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", host+"/api/chat", bytes.NewBuffer(jsonData))
	httpReq.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ollama error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	return &ControlBrainResponse{
		Content:   result.Message.Content,
		Model:     model,
		LatencyMs: time.Since(start).Milliseconds(),
		Local:     true,
	}, nil
}

// ─── Startup ─────────────────────────────────────────────────────────────────

// InitControlBrain starts Ollama and pulls the model in the background.
// Call once at worker startup. Does not block.
func InitControlBrain(ctx context.Context) {
	go func() {
		if err := ensureOllamaRunning(ctx); err != nil {
			log.Printf("⚠️ [ControlBrain] Ollama unavailable: %v", err)
			return
		}
		model := controlBrainModel()
		if err := pullModelIfNeeded(ctx, model); err != nil {
			log.Printf("⚠️ [ControlBrain] Model pull failed (%s): %v", model, err)
			return
		}
		soul := strings.TrimSpace(os.Getenv("SOUL_ID"))
		if soul == "" {
			soul = strings.TrimSpace(os.Getenv("AGENT_ID"))
		}
		log.Printf("✅ [ControlBrain] Ready — soul: %s model: %s @ %s", soul, model, ollamaHost())
	}()
}

// ─── Routing Stats ────────────────────────────────────────────────────────────

var (
	routingStats   = map[string]int64{"local": 0, "master": 0, "escalated": 0}
	routingStatsMu sync.Mutex
)

// RecordRouting increments routing counters for heartbeat reporting.
func RecordRouting(target BrainTarget, escalated bool) {
	routingStatsMu.Lock()
	defer routingStatsMu.Unlock()
	if escalated {
		routingStats["escalated"]++
	} else if target == BrainLocal {
		routingStats["local"]++
	} else {
		routingStats["master"]++
	}
}

// GetRoutingStats returns a snapshot of routing counters.
func GetRoutingStats() map[string]int64 {
	routingStatsMu.Lock()
	defer routingStatsMu.Unlock()
	snap := map[string]int64{}
	for k, v := range routingStats {
		snap[k] = v
	}
	return snap
}
