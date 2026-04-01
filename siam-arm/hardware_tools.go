package main

// hardware_tools.go — Phase 4B: Embodied Agent Hardware Access
//
// Provides camera, location, battery, and sensor tools for the Control Brain.
// Uses Termux:API on Android and standard Linux tools otherwise.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// HardwareInfo bundles all available sensor data for a snapshot.
type HardwareInfo struct {
	Battery  *BatteryInfo  `json:"battery,omitempty"`
	Location *LocationInfo `json:"location,omitempty"`
	Sensors  *SensorInfo   `json:"sensors,omitempty"`
	Platform string        `json:"platform"`
	Timestamp time.Time   `json:"timestamp"`
}

type BatteryInfo struct {
	Level    float64 `json:"level"`     // 0-100
	Plugged  bool    `json:"plugged"`
	Status   string  `json:"status"`   // "charging" | "discharging" | "full" | "unknown"
}

type LocationInfo struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Accuracy  float64 `json:"accuracy,omitempty"`
	Provider  string  `json:"provider,omitempty"` // "gps" | "network" | "passive"
}

type SensorInfo struct {
	Accelerometer []float64 `json:"accelerometer,omitempty"` // [x, y, z]
	Light         float64   `json:"light,omitempty"`         // lux
	Proximity     float64   `json:"proximity,omitempty"`
}

// ─── Battery ─────────────────────────────────────────────────────────────────

// GetBattery reads battery status.
func GetBattery(ctx context.Context) (*BatteryInfo, error) {
	if isTermux() {
		return termuxBattery(ctx)
	}
	return linuxBattery()
}

func termuxBattery(ctx context.Context) (*BatteryInfo, error) {
	out, err := exec.CommandContext(ctx, "termux-battery-status").Output()
	if err != nil {
		return nil, err
	}
	var raw struct {
		Percentage float64 `json:"percentage"`
		Plugged    string  `json:"plugged"`
		Status     string  `json:"status"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}
	return &BatteryInfo{
		Level:   raw.Percentage,
		Plugged: raw.Plugged != "UNPLUGGED",
		Status:  strings.ToLower(raw.Status),
	}, nil
}

func linuxBattery() (*BatteryInfo, error) {
	// Try /sys/class/power_supply/BAT0 (common on laptops)
	dirs := []string{"/sys/class/power_supply/BAT0", "/sys/class/power_supply/BAT1"}
	for _, dir := range dirs {
		capPath := filepath.Join(dir, "capacity")
		statusPath := filepath.Join(dir, "status")
		capBytes, err := os.ReadFile(capPath)
		if err != nil {
			continue
		}
		statusBytes, _ := os.ReadFile(statusPath)
		var level float64
		fmt.Sscanf(strings.TrimSpace(string(capBytes)), "%f", &level)
		status := strings.ToLower(strings.TrimSpace(string(statusBytes)))
		return &BatteryInfo{
			Level:   level,
			Plugged: status == "charging" || status == "full",
			Status:  status,
		}, nil
	}
	return nil, fmt.Errorf("no battery found")
}

// ─── Location ─────────────────────────────────────────────────────────────────

// GetLocation returns GPS/network location (Termux only; Linux returns error).
func GetLocation(ctx context.Context) (*LocationInfo, error) {
	if !isTermux() {
		return nil, fmt.Errorf("location not supported on this platform")
	}
	// Try GPS first, fall back to network
	for _, provider := range []string{"gps", "network", "passive"} {
		ctxT, cancel := context.WithTimeout(ctx, 10*time.Second)
		out, err := exec.CommandContext(ctxT, "termux-location", "-p", provider, "-r", "once").Output()
		cancel()
		if err != nil || len(out) == 0 {
			continue
		}
		var raw struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
			Accuracy  float64 `json:"accuracy"`
			Provider  string  `json:"provider"`
		}
		if json.Unmarshal(out, &raw) == nil && (raw.Latitude != 0 || raw.Longitude != 0) {
			return &LocationInfo{
				Latitude:  raw.Latitude,
				Longitude: raw.Longitude,
				Accuracy:  raw.Accuracy,
				Provider:  raw.Provider,
			}, nil
		}
	}
	return nil, fmt.Errorf("location unavailable")
}

// ─── Sensors ─────────────────────────────────────────────────────────────────

// GetSensors reads available sensor data (Termux only).
func GetSensors(ctx context.Context) (*SensorInfo, error) {
	if !isTermux() {
		return nil, fmt.Errorf("sensor API not available on this platform")
	}
	out, err := exec.CommandContext(ctx, "termux-sensor", "-s", "accelerometer,light,proximity", "-n", "1").Output()
	if err != nil {
		return nil, err
	}
	// Response is JSON like {"accelerometer":{"values":[x,y,z]},"light":{"values":[lux]}}
	var raw map[string]struct {
		Values []float64 `json:"values"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}
	info := &SensorInfo{}
	if accel, ok := raw["accelerometer"]; ok && len(accel.Values) >= 3 {
		info.Accelerometer = accel.Values[:3]
	}
	if light, ok := raw["light"]; ok && len(light.Values) > 0 {
		info.Light = light.Values[0]
	}
	if prox, ok := raw["proximity"]; ok && len(prox.Values) > 0 {
		info.Proximity = prox.Values[0]
	}
	return info, nil
}

// ─── Camera ──────────────────────────────────────────────────────────────────

// TakePhoto captures a photo and returns the file path.
// Uses termux-camera-photo on Android, ffmpeg/v4l2 on Linux.
func TakePhoto(parentCtx context.Context, outputPath string) (string, error) {
	// Prevent hardware timeouts from stalling the entire worker stream
	ctx, cancel := context.WithTimeout(parentCtx, 15*time.Second)
	defer cancel()

	if outputPath == "" {
		outputPath = fmt.Sprintf("/tmp/photo-%d.jpg", time.Now().UnixNano())
	}

	// High Priority: Check for IP Webcam (Great for PRoot/External Nodes)
	ipcam := os.Getenv("IP_WEBCAM_URL")
	if ipcam != "" {
		shotURL := ipcam
		if !strings.HasSuffix(shotURL, ".jpg") {
			shotURL = strings.TrimRight(ipcam, "/") + "/shot.jpg"
		}
		
		req, err := http.NewRequestWithContext(ctx, "GET", shotURL, nil)
		if err == nil {
			if resp, doErr := http.DefaultClient.Do(req); doErr == nil {
				defer resp.Body.Close()
				if out, createErr := os.Create(outputPath); createErr == nil {
					defer out.Close()
					io.Copy(out, resp.Body)
					return outputPath, nil
				}
			}
		}
		return "", fmt.Errorf("failed to capture from IP webcam: %v", shotURL)
	}

	if _, lookErr := exec.LookPath("termux-camera-photo"); lookErr == nil || isTermux() {
		// termux-camera-photo -c 0 <path>  (camera 0 = back, 1 = front)
		camera := os.Getenv("CAMERA_ID")
		if camera == "" {
			camera = "0"
		}
		err := exec.CommandContext(ctx, "termux-camera-photo", "-c", camera, outputPath).Run()
		if err != nil {
			return "", fmt.Errorf("termux-camera-photo failed: %w", err)
		}
		return outputPath, nil
	}

	// Linux: try ffmpeg with v4l2
	device := os.Getenv("VIDEO_DEVICE")
	if device == "" {
		device = "/dev/video0"
	}
	if _, err := os.Stat(device); err != nil {
		return "", fmt.Errorf("no camera device at %s", device)
	}
	err := exec.CommandContext(ctx,
		"ffmpeg", "-y", "-f", "v4l2", "-i", device,
		"-frames:v", "1", "-q:v", "2", outputPath,
	).Run()
	if err != nil {
		return "", fmt.Errorf("ffmpeg capture failed (or timed out): %w", err)
	}
	return outputPath, nil
}

// ─── Hardware Snapshot ───────────────────────────────────────────────────────

// GetHardwareSnapshot collects all available sensor data in parallel.
func GetHardwareSnapshot(ctx context.Context) HardwareInfo {
	info := HardwareInfo{
		Platform:  detectPlatform(),
		Timestamp: time.Now(),
	}

	// Battery (fast, always try)
	if bat, err := GetBattery(ctx); err == nil {
		info.Battery = bat
	}

	// Location (async, may be slow)
	locCh := make(chan *LocationInfo, 1)
	go func() {
		loc, _ := GetLocation(ctx)
		locCh <- loc
	}()

	// Sensors (Termux only, fast)
	if sensInfo, err := GetSensors(ctx); err == nil {
		info.Sensors = sensInfo
	}

	// Collect location with timeout
	select {
	case loc := <-locCh:
		info.Location = loc
	case <-time.After(5 * time.Second):
	}

	return info
}

// ─── Control Brain Tool Dispatcher ───────────────────────────────────────────

// ControlBrainTool maps tool names to handler functions callable by the Control Brain.
type ControlBrainTool struct {
	Name        string
	Description string
	Handler     func(ctx context.Context, args map[string]string) (string, error)
}

// GetHardwareTools returns the list of hardware tools available on this device.
func GetHardwareTools() []ControlBrainTool {
	tools := []ControlBrainTool{
		{
			Name:        "get_hardware_snapshot",
			Description: "Get battery, location, and sensor data from this device",
			Handler: func(ctx context.Context, args map[string]string) (string, error) {
				snap := GetHardwareSnapshot(ctx)
				b, _ := json.MarshalIndent(snap, "", "  ")
				return string(b), nil
			},
		},
		{
			Name:        "get_battery",
			Description: "Get battery level and charging status",
			Handler: func(ctx context.Context, args map[string]string) (string, error) {
				bat, err := GetBattery(ctx)
				if err != nil {
					return "", err
				}
				b, _ := json.Marshal(bat)
				return string(b), nil
			},
		},
		{
			Name:        "take_photo",
			Description: "Capture a photo from the device camera",
			Handler: func(ctx context.Context, args map[string]string) (string, error) {
				path, err := TakePhoto(ctx, args["output_path"])
				if err != nil {
					return "", err
				}
				return fmt.Sprintf(`{"path":"%s","success":true}`, path), nil
			},
		},
		{
			Name:        "analyze_photo",
			Description: "Capture or analyze a photo using the local vision model (LLaVA/moondream). Args: prompt (optional), image_path (optional, captures fresh photo if omitted)",
			Handler: func(ctx context.Context, args map[string]string) (string, error) {
				prompt := args["prompt"]
				imagePath := args["image_path"]
				return AnalyzePhoto(ctx, imagePath, prompt)
			},
		},
	}

	// Location only makes sense on Termux
	if isTermux() {
		tools = append(tools, ControlBrainTool{
			Name:        "get_location",
			Description: "Get GPS location of this device",
			Handler: func(ctx context.Context, args map[string]string) (string, error) {
				loc, err := GetLocation(ctx)
				if err != nil {
					return "", err
				}
				b, _ := json.Marshal(loc)
				return string(b), nil
			},
		})
		tools = append(tools, ControlBrainTool{
			Name:        "get_sensors",
			Description: "Read accelerometer, light, and proximity sensors",
			Handler: func(ctx context.Context, args map[string]string) (string, error) {
				sens, err := GetSensors(ctx)
				if err != nil {
					return "", err
				}
				b, _ := json.Marshal(sens)
				return string(b), nil
			},
		})
	}

	return tools
}

// ToolDescriptionsJSON returns JSON-serializable tool descriptions for the Control Brain.
func ToolDescriptionsJSON() string {
	type toolDesc struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	tools := GetHardwareTools()
	descs := make([]toolDesc, len(tools))
	for i, t := range tools {
		descs[i] = toolDesc{Name: t.Name, Description: t.Description}
	}
	b, _ := json.Marshal(descs)
	return string(b)
}

// DispatchHardwareTool executes a named hardware tool and returns result JSON.
func DispatchHardwareTool(ctx context.Context, toolName string, args map[string]string) (string, error) {
	for _, t := range GetHardwareTools() {
		if t.Name == toolName {
			return t.Handler(ctx, args)
		}
	}
	return "", fmt.Errorf("unknown tool: %s", toolName)
}

// ─── SYSTEM_BRAIN_QUERY handler helper ───────────────────────────────────────

// HandleBrainQuery processes a SYSTEM_BRAIN_QUERY command from master.
// Payload format: "<prompt>" or JSON {"prompt":"...","system":"...","tools":true}
func HandleBrainQuery(ctx context.Context, payload string) string {
	var prompt, system string
	wantsTools := false

	// Try JSON parse first
	var req struct {
		Prompt  string `json:"prompt"`
		System  string `json:"system"`
		Tools   bool   `json:"tools"`
	}
	if err := json.Unmarshal([]byte(payload), &req); err == nil {
		prompt = req.Prompt
		system = req.System
		wantsTools = req.Tools
	} else {
		prompt = payload
	}

	if prompt == "" {
		return `{"error":"empty prompt"}`
	}

	// Inject relevant long-term memories into system prompt
	if memCtx := fetchMemoryContext(ctx, prompt); memCtx != "" {
		system += "\n\n[Long-Term Memory]\n" + memCtx
	}

	// Inject available hardware tools into system prompt if requested
	if wantsTools {
		toolsDesc := ToolDescriptionsJSON()
		system += fmt.Sprintf("\n\nAvailable hardware tools (call by replying JSON {\"tool\":\"name\",\"args\":{}}): %s", toolsDesc)
	}

	resp, err := QueryControlBrain(ctx, system, prompt)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}

	// Check if Control Brain requests escalation to Thinking Brain
	if strings.Contains(resp.Content, "[[ESCALATE]]") {
		RecordRouting(BrainMaster, true)
		return `{"escalate":true,"reason":"control_brain_requested_escalation"}`
	}

	// Check if response is a tool call
	if strings.HasPrefix(strings.TrimSpace(resp.Content), `{"tool"`) {
		var toolCall struct {
			Tool string            `json:"tool"`
			Args map[string]string `json:"args"`
		}
		if json.Unmarshal([]byte(strings.TrimSpace(resp.Content)), &toolCall) == nil {
			result, toolErr := DispatchHardwareTool(ctx, toolCall.Tool, toolCall.Args)
			if toolErr != nil {
				result = fmt.Sprintf(`{"error":%q}`, toolErr.Error())
			}
			RecordRouting(BrainLocal, false)
			out, _ := json.Marshal(map[string]interface{}{
				"content":    result,
				"tool_used":  toolCall.Tool,
				"local":      true,
				"latency_ms": resp.LatencyMs,
			})
			return string(out)
		}
	}

	RecordRouting(BrainLocal, false)
	// Build a var to avoid composite literal issue
	resultMap := map[string]interface{}{
		"content":    resp.Content,
		"model":      resp.Model,
		"local":      true,
		"latency_ms": resp.LatencyMs,
	}
	out, _ := json.Marshal(resultMap)

	// Suppress unused import warning for bytes if not used elsewhere
	_ = bytes.NewBuffer
	return string(out)
}

// ─── Long-Term Memory (Phase 6B) ─────────────────────────────────────────────

// storeMemoryOnMaster sends a SYSTEM_REMEMBER payload to master's memory API.
// payload: JSON string with content, importance, tags fields.
func storeMemoryOnMaster(ctx context.Context, payload string) string {
	agentID := strings.TrimSpace(os.Getenv("AGENT_ID"))
	masterURL := masterHTTPBase() + "/agents/" + agentID + "/memory"
	apiKey := os.Getenv("MASTER_API_KEY")

	// If payload is not JSON, wrap it as content
	if !strings.HasPrefix(strings.TrimSpace(payload), "{") {
		b, _ := json.Marshal(map[string]interface{}{
			"content":    payload,
			"importance": 0.6,
		})
		payload = string(b)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", masterURL, strings.NewReader(payload))
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	defer resp.Body.Close()
	out := make([]byte, 512)
	n, _ := resp.Body.Read(out)
	return string(out[:n])
}

// recallMemoryFromMaster searches agent memories on master.
// payload: query string or JSON {"q":"...","limit":5}
func recallMemoryFromMaster(ctx context.Context, payload string) string {
	agentID := strings.TrimSpace(os.Getenv("AGENT_ID"))
	masterURL := masterHTTPBase()
	apiKey := os.Getenv("MASTER_API_KEY")

	var query string
	var limit = 5
	if strings.HasPrefix(strings.TrimSpace(payload), "{") {
		var p struct {
			Q     string `json:"q"`
			Limit int    `json:"limit"`
		}
		if json.Unmarshal([]byte(payload), &p) == nil {
			query = p.Q
			if p.Limit > 0 {
				limit = p.Limit
			}
		}
	} else {
		query = payload
	}

	url := fmt.Sprintf("%s/agents/%s/memory/search?q=%s&limit=%d",
		masterURL, agentID, strings.ReplaceAll(query, " ", "+"), limit)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	req.Header.Set("X-API-Key", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	defer resp.Body.Close()
	out := make([]byte, 4096)
	n, _ := resp.Body.Read(out)
	return string(out[:n])
}

// fetchMemoryContext fetches the top-N memory context string from master for
// injection into the Control Brain system prompt.
func fetchMemoryContext(ctx context.Context, query string) string {
	agentID := strings.TrimSpace(os.Getenv("AGENT_ID"))
	if agentID == "" {
		return ""
	}
	masterURL := masterHTTPBase()
	apiKey := os.Getenv("MASTER_API_KEY")

	url := fmt.Sprintf("%s/agents/%s/memory/context?q=%s&limit=4",
		masterURL, agentID, strings.ReplaceAll(query, " ", "+"))
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("X-API-Key", apiKey)

	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var result struct {
		Context string `json:"context"`
	}
	out := make([]byte, 4096)
	n, _ := resp.Body.Read(out)
	if json.Unmarshal(out[:n], &result) == nil {
		return result.Context
	}
	return ""
}

// ─── ORCHESTRATOR_STEP handler ────────────────────────────────────────────────

// handleOrchestratorStep processes an ORCHESTRATOR_STEP command from master.
// Payload: JSON {"job_id":"...","step_id":"...","skill":"...","input":"..."}
// Dispatches to a hardware tool if the skill matches, otherwise forwards to
// the Control Brain for LLM-based execution.
func handleOrchestratorStep(ctx context.Context, payload string) string {
	var req struct {
		JobID   string `json:"job_id"`
		StepID  string `json:"step_id"`
		Skill   string `json:"skill"`
		Input   string `json:"input"` // JSON string of key→value map
	}
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		return fmt.Sprintf(`{"error":"invalid payload: %s"}`, err.Error())
	}

	// Parse input map
	var inputMap map[string]string
	if req.Input != "" {
		json.Unmarshal([]byte(req.Input), &inputMap) //nolint:errcheck
	}
	if inputMap == nil {
		inputMap = map[string]string{}
	}

	// Try hardware tool dispatch first
	result, err := DispatchHardwareTool(ctx, req.Skill, inputMap)
	if err == nil {
		out, _ := json.Marshal(map[string]interface{}{
			"job_id":  req.JobID,
			"step_id": req.StepID,
			"result":  result,
			"source":  "hardware_tool",
		})
		return string(out)
	}

	// Fall back to Control Brain
	prompt := fmt.Sprintf("Execute skill: %s\nInput: %s", req.Skill, req.Input)
	brainResp, brainErr := QueryControlBrain(ctx, "", prompt)
	if brainErr != nil {
		return fmt.Sprintf(`{"error":"skill %q failed: %s","job_id":%q,"step_id":%q}`,
			req.Skill, brainErr.Error(), req.JobID, req.StepID)
	}
	RecordRouting(BrainLocal, false)
	out, _ := json.Marshal(map[string]interface{}{
		"job_id":     req.JobID,
		"step_id":    req.StepID,
		"result":     brainResp.Content,
		"source":     "control_brain",
		"latency_ms": brainResp.LatencyMs,
	})
	return string(out)
}
