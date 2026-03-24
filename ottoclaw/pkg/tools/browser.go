package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// BrowserLaunchTool opens a URL in a browser on the local machine.
// Requires a GUI environment (DISPLAY must be set on Linux).
// Supports any installed browser: xdg-open (default), chromium, firefox, brave-browser, etc.
type BrowserLaunchTool struct {
	defaultBrowser  string   // binary name or path, empty = xdg-open
	allowedBrowsers []string // whitelist; empty = allow any
}

func NewBrowserLaunchTool(defaultBrowser string, allowedBrowsers []string) *BrowserLaunchTool {
	return &BrowserLaunchTool{
		defaultBrowser:  defaultBrowser,
		allowedBrowsers: allowedBrowsers,
	}
}

func (t *BrowserLaunchTool) Name() string { return "open_browser" }

func (t *BrowserLaunchTool) Description() string {
	return "Open a URL in a browser on the local machine. " +
		"Requires a GUI display (Linux: DISPLAY must be set, e.g. Zorin OS / Ubuntu Desktop). " +
		"Supported browsers: chromium, google-chrome, firefox, brave-browser, or system default (xdg-open)."
}

func (t *BrowserLaunchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "The URL to open (e.g. https://example.com)",
			},
			"browser": map[string]any{
				"type":        "string",
				"description": "Browser binary to use: chromium, google-chrome, firefox, brave-browser, or leave empty to use the system default (xdg-open).",
			},
		},
		"required": []string{"url"},
	}
}

func (t *BrowserLaunchTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	url, ok := args["url"].(string)
	if !ok || strings.TrimSpace(url) == "" {
		return ErrorResult("url is required")
	}
	url = strings.TrimSpace(url)

	// Basic URL sanity check
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") &&
		!strings.HasPrefix(url, "file://") {
		return ErrorResult(fmt.Sprintf("invalid URL %q — must start with http://, https://, or file://", url))
	}

	// Determine which browser to use
	browserBin := ""
	if b, ok := args["browser"].(string); ok {
		browserBin = strings.TrimSpace(b)
	}
	if browserBin == "" || browserBin == "default" {
		browserBin = t.defaultBrowser
	}

	// Allowlist check (if configured)
	if len(t.allowedBrowsers) > 0 && browserBin != "" {
		allowed := false
		for _, a := range t.allowedBrowsers {
			if strings.EqualFold(a, browserBin) {
				allowed = true
				break
			}
		}
		if !allowed {
			return ErrorResult(fmt.Sprintf(
				"browser %q is not in the allowed list: %s",
				browserBin, strings.Join(t.allowedBrowsers, ", "),
			))
		}
	}

	// Platform-specific launch
	cmd, err := buildLaunchCmd(ctx, url, browserBin)
	if err != nil {
		return ErrorResult(err.Error())
	}

	// Detach from parent process — browser runs independently
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return ErrorResult(fmt.Sprintf("failed to launch browser: %v", err))
	}

	// Capture PID before goroutine may call Wait() and release process resources
	pid := cmd.Process.Pid

	// Don't wait — browser is a GUI app, release it immediately
	go func() { _ = cmd.Wait() }()

	used := browserBin
	if used == "" {
		used = defaultLauncherName()
	}

	return NewToolResult(fmt.Sprintf("Opened %s in %s (PID %d)", url, used, pid))
}

// buildLaunchCmd constructs the OS command for opening the URL in a browser.
func buildLaunchCmd(ctx context.Context, url, browserBin string) (*exec.Cmd, error) {
	switch runtime.GOOS {
	case "linux":
		return linuxLaunchCmd(ctx, url, browserBin)
	case "darwin":
		if browserBin != "" {
			return exec.CommandContext(ctx, browserBin, url), nil
		}
		return exec.CommandContext(ctx, "open", url), nil
	case "windows":
		if browserBin != "" {
			return exec.CommandContext(ctx, browserBin, url), nil
		}
		return exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", url), nil
	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// linuxLaunchCmd handles Linux-specific browser launching.
// On a GUI session (DISPLAY or WAYLAND_DISPLAY is set), it uses xdg-open or the named browser.
// On headless environments (no display), it returns a clear error.
func linuxLaunchCmd(ctx context.Context, url, browserBin string) (*exec.Cmd, error) {
	// Check for a display server
	display := os.Getenv("DISPLAY")
	waylandDisplay := os.Getenv("WAYLAND_DISPLAY")
	if display == "" && waylandDisplay == "" {
		return nil, fmt.Errorf(
			"no GUI display detected (DISPLAY and WAYLAND_DISPLAY are both unset). " +
				"This tool requires a graphical environment such as Zorin OS / Ubuntu Desktop. " +
				"On headless servers, use web_fetch or web_search instead.",
		)
	}

	if browserBin == "" {
		// Use xdg-open — respects user's default browser setting
		if _, err := exec.LookPath("xdg-open"); err != nil {
			return nil, fmt.Errorf("xdg-open not found; install with: sudo apt install xdg-utils")
		}
		return exec.CommandContext(ctx, "xdg-open", url), nil
	}

	// Try to find the specified browser in PATH
	path, err := exec.LookPath(browserBin)
	if err != nil {
		// Provide helpful message for known browsers
		hint := installHint(browserBin)
		return nil, fmt.Errorf("browser %q not found in PATH%s", browserBin, hint)
	}
	return exec.CommandContext(ctx, path, url), nil
}

// defaultLauncherName returns a human-readable name for the system default launcher.
func defaultLauncherName() string {
	switch runtime.GOOS {
	case "linux":
		return "xdg-open (system default browser)"
	case "darwin":
		return "open (macOS default browser)"
	case "windows":
		return "rundll32 (Windows default browser)"
	default:
		return "system default"
	}
}

// installHint returns an install suggestion for known browsers.
func installHint(name string) string {
	hints := map[string]string{
		"chromium":      " — install with: sudo apt install chromium-browser",
		"google-chrome": " — download from https://www.google.com/chrome",
		"firefox":       " — install with: sudo apt install firefox",
		"brave-browser": " — install from https://brave.com/linux",
	}
	if h, ok := hints[strings.ToLower(name)]; ok {
		return h
	}
	return ""
}
