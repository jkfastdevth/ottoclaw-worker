package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// RishTool controls an Android device via Shizuku's rish shell.
// rish provides ADB-level shell access without USB — requires Shizuku running on device.
//
// Actions:
//   tap, swipe, keyevent, type_text          — UI control
//   screencap                                 — screenshot to /sdcard
//   open_app                                  — launch app by package or component
//   get_focus                                 — get currently focused activity
//   wifi                                      — enable/disable Wi-Fi
//   shell                                     — raw rish command (for advanced use)
//   check                                     — verify rish is available + permissions

type RishTool struct{}

func NewRishTool() *RishTool {
	return &RishTool{}
}

func (t *RishTool) Name() string {
	return "android_rish"
}

func (t *RishTool) Description() string {
	return "Control an Android device using Shizuku's rish shell (ADB-level access from Termux). " +
		"Actions: tap (touch screen), swipe (gesture), keyevent (hardware key), type_text (input text), " +
		"screencap (screenshot), open_app (launch app), get_focus (current app), wifi (on/off), " +
		"shell (raw command), check (verify rish available). " +
		"Requires: Shizuku app running on Android device."
}

func (t *RishTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type": "string",
				"enum": []string{
					"tap", "swipe", "keyevent", "type_text",
					"screencap", "open_app", "get_focus",
					"wifi", "shell", "check",
				},
				"description": "Action to perform on the Android device",
			},
			// tap / swipe
			"x": map[string]any{
				"type":        "integer",
				"description": "X coordinate in pixels (for tap, swipe start)",
			},
			"y": map[string]any{
				"type":        "integer",
				"description": "Y coordinate in pixels (for tap, swipe start)",
			},
			"x2": map[string]any{
				"type":        "integer",
				"description": "Swipe end X coordinate",
			},
			"y2": map[string]any{
				"type":        "integer",
				"description": "Swipe end Y coordinate",
			},
			"duration_ms": map[string]any{
				"type":        "integer",
				"description": "Swipe duration in milliseconds (default: 400)",
			},
			// keyevent
			"keycode": map[string]any{
				"type": "integer",
				"description": "Android keycode. Common: 3=Home, 4=Back, 26=Power, 82=Menu, " +
					"24=Vol+, 25=Vol-, 66=Enter, 67=Backspace, 187=Recents, 223=Sleep",
			},
			// type_text
			"text": map[string]any{
				"type":        "string",
				"description": "Text to type (spaces must be underscores or use %s format). Avoid special chars.",
			},
			// screencap
			"path": map[string]any{
				"type":        "string",
				"description": "Output path for screenshot (default: /sdcard/screen.png)",
			},
			// open_app
			"package": map[string]any{
				"type": "string",
				"description": "App package name e.g. com.facebook.katana, com.google.android.youtube. " +
					"Or full component: com.android.settings/.Settings",
			},
			// wifi
			"enabled": map[string]any{
				"type":        "boolean",
				"description": "true=enable Wi-Fi, false=disable Wi-Fi",
			},
			// shell (raw)
			"command": map[string]any{
				"type":        "string",
				"description": "Raw rish command to execute (for advanced use, e.g. 'dumpsys battery')",
			},
		},
		"required": []string{"action"},
	}
}

func (t *RishTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	action, _ := args["action"].(string)

	switch action {
	case "check":
		return t.check(ctx)
	case "tap":
		return t.tap(ctx, args)
	case "swipe":
		return t.swipe(ctx, args)
	case "keyevent":
		return t.keyevent(ctx, args)
	case "type_text":
		return t.typeText(ctx, args)
	case "screencap":
		return t.screencap(ctx, args)
	case "open_app":
		return t.openApp(ctx, args)
	case "get_focus":
		return t.getFocus(ctx)
	case "wifi":
		return t.wifi(ctx, args)
	case "shell":
		return t.rawShell(ctx, args)
	default:
		return ErrorResult("unknown action: " + action)
	}
}

// rish executes a command via rish shell with timeout.
func (t *RishTool) rish(ctx context.Context, command string, timeout time.Duration) (string, error) {
	ctx2, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx2, "rish", "-c", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	out := strings.TrimSpace(stdout.String())
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "rish: ") {
			return "", fmt.Errorf("rish not found — install Shizuku and run: rish (from Termux)")
		}
		if out != "" {
			return out, nil // some commands return output even on non-zero exit
		}
		if errMsg != "" {
			return "", fmt.Errorf("%s", errMsg)
		}
		return "", err
	}
	return out, nil
}

func (t *RishTool) check(ctx context.Context) *ToolResult {
	out, err := t.rish(ctx, "id && pm list packages | wc -l", 5*time.Second)
	if err != nil {
		return ErrorResult(fmt.Sprintf("rish unavailable: %v\nFix: ensure Shizuku is running and run `rish` once from Termux to authorize", err))
	}
	return UserResult(fmt.Sprintf("✅ rish OK\n%s", out))
}

func (t *RishTool) tap(ctx context.Context, args map[string]any) *ToolResult {
	x, y, ok := getCoords(args, "x", "y")
	if !ok {
		return ErrorResult("tap requires x and y coordinates")
	}
	cmd := fmt.Sprintf("input tap %d %d", x, y)
	out, err := t.rish(ctx, cmd, 5*time.Second)
	if err != nil {
		return ErrorResult(fmt.Sprintf("tap failed: %v", err))
	}
	msg := fmt.Sprintf("✅ Tapped (%d, %d)", x, y)
	if out != "" {
		msg += "\n" + out
	}
	return UserResult(msg)
}

func (t *RishTool) swipe(ctx context.Context, args map[string]any) *ToolResult {
	x1, y1, ok1 := getCoords(args, "x", "y")
	x2, y2, ok2 := getCoords(args, "x2", "y2")
	if !ok1 || !ok2 {
		return ErrorResult("swipe requires x, y (start) and x2, y2 (end)")
	}
	ms := 400
	if d, ok := args["duration_ms"].(float64); ok && d > 0 {
		ms = int(d)
	}
	cmd := fmt.Sprintf("input swipe %d %d %d %d %d", x1, y1, x2, y2, ms)
	_, err := t.rish(ctx, cmd, 10*time.Second)
	if err != nil {
		return ErrorResult(fmt.Sprintf("swipe failed: %v", err))
	}
	return UserResult(fmt.Sprintf("✅ Swiped (%d,%d) → (%d,%d) in %dms", x1, y1, x2, y2, ms))
}

func (t *RishTool) keyevent(ctx context.Context, args map[string]any) *ToolResult {
	kc, ok := args["keycode"].(float64)
	if !ok {
		return ErrorResult("keycode is required (integer)")
	}
	keyNames := map[int]string{
		3: "Home", 4: "Back", 24: "Vol+", 25: "Vol-", 26: "Power",
		66: "Enter", 67: "Backspace", 82: "Menu", 187: "Recents", 223: "Sleep",
	}
	name := keyNames[int(kc)]
	if name == "" {
		name = strconv.Itoa(int(kc))
	}
	cmd := fmt.Sprintf("input keyevent %d", int(kc))
	_, err := t.rish(ctx, cmd, 5*time.Second)
	if err != nil {
		return ErrorResult(fmt.Sprintf("keyevent failed: %v", err))
	}
	return UserResult(fmt.Sprintf("✅ Key: %s (%d)", name, int(kc)))
}

func (t *RishTool) typeText(ctx context.Context, args map[string]any) *ToolResult {
	text, ok := args["text"].(string)
	if !ok || text == "" {
		return ErrorResult("text is required")
	}
	// rish input text requires spaces as %s and no special shell chars
	// Safe: only allow alphanumeric + underscore + dash + dot
	safe := regexp.MustCompile(`[^a-zA-Z0-9._\-]`)
	safed := safe.ReplaceAllString(text, "_")
	cmd := fmt.Sprintf("input text '%s'", safed)
	_, err := t.rish(ctx, cmd, 5*time.Second)
	if err != nil {
		return ErrorResult(fmt.Sprintf("type_text failed: %v", err))
	}
	return UserResult(fmt.Sprintf("✅ Typed: %s", safed))
}

func (t *RishTool) screencap(ctx context.Context, args map[string]any) *ToolResult {
	path := "/sdcard/screen.png"
	if p, ok := args["path"].(string); ok && p != "" {
		path = p
	}
	cmd := fmt.Sprintf("screencap -p %s", path)
	_, err := t.rish(ctx, cmd, 10*time.Second)
	if err != nil {
		return ErrorResult(fmt.Sprintf("screencap failed: %v", err))
	}
	// Verify file exists
	check, _ := t.rish(ctx, fmt.Sprintf("ls -lh %s", path), 3*time.Second)
	return UserResult(fmt.Sprintf("✅ Screenshot saved: %s\n%s", path, check))
}

func (t *RishTool) openApp(ctx context.Context, args map[string]any) *ToolResult {
	pkg, ok := args["package"].(string)
	if !ok || pkg == "" {
		return ErrorResult("package is required (e.g. com.facebook.katana or com.android.settings/.Settings)")
	}

	var cmd string
	if strings.Contains(pkg, "/") {
		// Full component: com.android.settings/.Settings
		cmd = fmt.Sprintf("am start -n %s", pkg)
	} else {
		// Package only — use monkey launcher
		cmd = fmt.Sprintf("monkey -p %s -c android.intent.category.LAUNCHER 1", pkg)
	}

	out, err := t.rish(ctx, cmd, 10*time.Second)
	if err != nil {
		return ErrorResult(fmt.Sprintf("open_app failed: %v", err))
	}
	return UserResult(fmt.Sprintf("✅ Launched: %s\n%s", pkg, out))
}

func (t *RishTool) getFocus(ctx context.Context) *ToolResult {
	out, err := t.rish(ctx, "dumpsys activity | grep mResumedActivity", 5*time.Second)
	if err != nil || out == "" {
		// Try alternative for newer Android
		out, err = t.rish(ctx, "dumpsys window | grep mCurrentFocus", 5*time.Second)
		if err != nil {
			return ErrorResult(fmt.Sprintf("get_focus failed: %v", err))
		}
	}
	return UserResult(fmt.Sprintf("📱 Current focus:\n%s", out))
}

func (t *RishTool) wifi(ctx context.Context, args map[string]any) *ToolResult {
	enabled, ok := args["enabled"].(bool)
	if !ok {
		return ErrorResult("enabled (boolean) is required")
	}
	state := "enable"
	if !enabled {
		state = "disable"
	}
	_, err := t.rish(ctx, fmt.Sprintf("svc wifi %s", state), 5*time.Second)
	if err != nil {
		return ErrorResult(fmt.Sprintf("wifi %s failed: %v", state, err))
	}
	icon := "📶"
	if !enabled {
		icon = "📵"
	}
	return UserResult(fmt.Sprintf("%s Wi-Fi %sd", icon, state))
}

func (t *RishTool) rawShell(ctx context.Context, args map[string]any) *ToolResult {
	command, ok := args["command"].(string)
	if !ok || command == "" {
		return ErrorResult("command is required for shell action")
	}
	// Block the most dangerous patterns
	lower := strings.ToLower(command)
	dangerous := []string{"reboot", "rm -rf", "mkfs", "dd if=", "format "}
	for _, d := range dangerous {
		if strings.Contains(lower, d) {
			return ErrorResult(fmt.Sprintf("command blocked: contains dangerous pattern '%s'. Use specific actions instead.", d))
		}
	}
	out, err := t.rish(ctx, command, 15*time.Second)
	if err != nil {
		return ErrorResult(fmt.Sprintf("shell failed: %v", err))
	}
	if out == "" {
		out = "(no output)"
	}
	maxLen := 3000
	if len(out) > maxLen {
		out = out[:maxLen] + fmt.Sprintf("\n... (truncated, %d more chars)", len(out)-maxLen)
	}
	return UserResult(out)
}

// getCoords extracts integer x,y coordinates from args (JSON numbers come as float64).
func getCoords(args map[string]any, xKey, yKey string) (int, int, bool) {
	xf, okX := args[xKey].(float64)
	yf, okY := args[yKey].(float64)
	if !okX || !okY {
		return 0, 0, false
	}
	return int(xf), int(yf), true
}
