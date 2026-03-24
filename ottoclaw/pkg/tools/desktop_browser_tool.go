package tools

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

//go:embed playwright_bridge.py
var playwrightBridgeScript []byte

// DesktopBrowserTool controls a real browser on a GUI desktop via Playwright.
// Works on Linux (X11/Wayland), macOS, and Windows wherever Playwright + Chromium is installed.
// Automatically switches between headed (DISPLAY set) and headless mode.
type DesktopBrowserTool struct{}

func NewDesktopBrowserTool() *DesktopBrowserTool {
	return &DesktopBrowserTool{}
}

func (t *DesktopBrowserTool) Name() string {
	return "desktop_browser"
}

func (t *DesktopBrowserTool) Description() string {
	return "Control a real web browser (Chromium) on this desktop machine using Playwright. " +
		"Supports: navigate (open URL), click (element or coordinates), type_text (fill forms), " +
		"screenshot (capture page), get_text (read content), get_url, wait_for (element), " +
		"evaluate_js (run JavaScript), scroll, press_key, select_option, hover. " +
		"Session (cookies/login state) is persisted automatically between calls. " +
		"Requires: python3 + playwright (pip install playwright && playwright install chromium). " +
		"Use session_id to maintain separate browser sessions (e.g. different accounts)."
}

func (t *DesktopBrowserTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"steps": map[string]any{
				"type":        "array",
				"description": "Sequence of browser actions to perform in a single browser session. Each step: {action, ...params}. Recommended: batch related actions together.",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type": "string",
							"enum": []string{
								"navigate", "click", "type_text", "screenshot",
								"get_text", "get_url", "wait_for", "evaluate_js",
								"scroll", "press_key", "select_option", "hover",
							},
						},
						"url":        map[string]any{"type": "string", "description": "URL to navigate to (navigate action)"},
						"selector":   map[string]any{"type": "string", "description": "CSS selector for element (click, type_text, get_text, wait_for, hover, select_option)"},
						"text":       map[string]any{"type": "string", "description": "Text to type (type_text action)"},
						"path":       map[string]any{"type": "string", "description": "File path for screenshot output (default: /tmp/ottoclaw_screenshot.png)"},
						"x":          map[string]any{"type": "number", "description": "X coordinate for click"},
						"y":          map[string]any{"type": "number", "description": "Y coordinate for click"},
						"js":         map[string]any{"type": "string", "description": "JavaScript expression to evaluate"},
						"key":        map[string]any{"type": "string", "description": "Key to press e.g. Enter, Tab, Escape, ArrowDown"},
						"value":      map[string]any{"type": "string", "description": "Option value to select (select_option action)"},
						"delta_y":    map[string]any{"type": "number", "description": "Scroll amount vertically in pixels (scroll action)"},
						"timeout_ms": map[string]any{"type": "number", "description": "Wait timeout in ms (wait_for action, default: 10000)"},
						"full_page":  map[string]any{"type": "boolean", "description": "Capture full page screenshot (screenshot action)"},
						"max_len":    map[string]any{"type": "number", "description": "Max characters to return from get_text (default: 3000)"},
					},
					"required": []string{"action"},
				},
			},
			"session_id": map[string]any{
				"type":        "string",
				"description": "Session name to persist cookies/login between calls. Default: 'default'. Use different IDs for different accounts.",
			},
			"headless": map[string]any{
				"type":        "boolean",
				"description": "Force headless mode (no visible window). Default: auto-detect based on DISPLAY env.",
			},
			"continue_on_error": map[string]any{
				"type":        "boolean",
				"description": "Continue executing remaining steps even if one step fails. Default: false (stop on first error).",
			},
		},
		"required": []string{"steps"},
	}
}

func (t *DesktopBrowserTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	// Verify Python3 + Playwright available
	python3, err := findPython3()
	if err != nil {
		return ErrorResult("Python3 not found: " + err.Error() +
			"\nFix: ensure python3 is in PATH")
	}

	// Ensure bridge script is available
	bridgePath, err := ensureBridgeScript()
	if err != nil {
		return ErrorResult("Failed to install playwright bridge: " + err.Error())
	}

	// Build payload for bridge
	sessionID := "default"
	if id, ok := args["session_id"].(string); ok && id != "" {
		sessionID = id
	}

	home, _ := os.UserHomeDir()
	sessionFile := filepath.Join(home, ".ottoclaw", "browser_sessions", sessionID+".json")
	if err := os.MkdirAll(filepath.Dir(sessionFile), 0700); err != nil {
		return ErrorResult("Failed to create session dir: " + err.Error())
	}

	payload := map[string]any{
		"steps":        args["steps"],
		"session_file": sessionFile,
	}
	if h, ok := args["headless"].(bool); ok {
		payload["headless"] = h
	}
	if c, ok := args["continue_on_error"].(bool); ok {
		payload["continue_on_error"] = c
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return ErrorResult("Failed to serialize payload: " + err.Error())
	}

	// Run bridge with timeout
	timeout := 120 * time.Second
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, python3, bridgePath)
	cmd.Stdin = bytes.NewReader(payloadJSON)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		if strings.Contains(stderrStr, "playwright") && strings.Contains(stderrStr, "not found") {
			return ErrorResult("Playwright not installed.\nFix: pip install playwright && playwright install chromium")
		}
		return ErrorResult(fmt.Sprintf("Browser bridge failed: %v\n%s", err, stderrStr))
	}

	// Parse and format output
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return ErrorResult("Failed to parse bridge output: " + err.Error() +
			"\nRaw: " + stdout.String()[:min(500, len(stdout.String()))])
	}

	if errMsg, ok := result["error"].(string); ok {
		tb, _ := result["traceback"].(string)
		return ErrorResult(fmt.Sprintf("Browser error: %s\n%s", errMsg, tb))
	}

	// Format results for agent
	output := formatBrowserResults(result)
	return UserResult(output)
}

// findPython3 locates the python3 executable.
func findPython3() (string, error) {
	for _, name := range []string{"python3", "python"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("python3 not found in PATH")
}

// ensureBridgeScript writes the embedded bridge script to ~/.ottoclaw/scripts/ if not present.
func ensureBridgeScript() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".ottoclaw", "scripts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "playwright_bridge.py")

	// Write/update the script (always overwrite to get latest version)
	if err := os.WriteFile(path, playwrightBridgeScript, 0755); err != nil {
		return "", err
	}
	return path, nil
}

func formatBrowserResults(result map[string]any) string {
	success, _ := result["success"].(bool)
	icon := "✅"
	if !success {
		icon = "❌"
	}

	steps, _ := result["results"].([]any)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s Browser session completed (%d steps)\n", icon, len(steps)))

	for i, s := range steps {
		step, ok := s.(map[string]any)
		if !ok {
			continue
		}
		action, _ := step["action"].(string)
		ok2, _ := step["ok"].(bool)
		statusIcon := "✅"
		if !ok2 {
			statusIcon = "❌"
		}

		sb.WriteString(fmt.Sprintf("\n[%d] %s %s", i+1, statusIcon, action))

		switch action {
		case "navigate":
			if url, ok := step["url"].(string); ok {
				sb.WriteString(fmt.Sprintf(" → %s", url))
			}
			if title, ok := step["title"].(string); ok {
				sb.WriteString(fmt.Sprintf(" (%q)", title))
			}
		case "screenshot":
			if path, ok := step["path"].(string); ok {
				sb.WriteString(fmt.Sprintf(" saved: %s", path))
			}
		case "type_text":
			if n, ok := step["chars"].(float64); ok {
				sb.WriteString(fmt.Sprintf(" (%d chars)", int(n)))
			}
		case "get_text":
			if text, ok := step["text"].(string); ok {
				preview := text
				if len(preview) > 200 {
					preview = preview[:200] + "..."
				}
				sb.WriteString(fmt.Sprintf("\n    %s", preview))
			}
		case "evaluate_js":
			if val, ok := step["result"].(string); ok {
				sb.WriteString(fmt.Sprintf(" = %s", val))
			}
		case "get_url":
			if url, ok := step["url"].(string); ok {
				sb.WriteString(fmt.Sprintf(": %s", url))
			}
		}

		if errMsg, ok := step["error"].(string); ok {
			sb.WriteString(fmt.Sprintf("\n    Error: %s", errMsg))
		}
	}

	return sb.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
