package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FacebookTool lets agents post to Facebook using the existing Playwright bridge.
// It abstracts the low-level browser steps into a single high-level call.
// Session is persisted at ~/.ottoclaw/browser_sessions/facebook.json
//
// Actions:
//   - post     — create a new post on the user's feed
//   - story    — placeholder (not yet implemented)
//   - save_session — open a headed browser for the user to login manually
type FacebookTool struct{}

func NewFacebookTool() *FacebookTool {
	return &FacebookTool{}
}

func (t *FacebookTool) Name() string { return "facebook" }

func (t *FacebookTool) Description() string {
	return "Post messages to Facebook on behalf of the user. " +
		"Actions: " +
		"'post' — create a new post on the user's personal Facebook feed (message required). " +
		"'save_session' — open a browser window for the user to login to Facebook and save the session (run once before posting). " +
		"Session cookies are stored persistently so login is only needed once. " +
		"Requires Playwright to be installed on this node."
}

func (t *FacebookTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"post", "save_session"},
				"description": "'post' to publish a message to the Facebook feed. 'save_session' to open browser for manual login.",
			},
			"message": map[string]any{
				"type":        "string",
				"description": "Text content to post on Facebook. Required when action is 'post'.",
			},
			"headless": map[string]any{
				"type":        "boolean",
				"description": "Force headless mode. Default: false for save_session, true for post.",
			},
		},
		"required": []string{"action"},
	}
}

func (t *FacebookTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	action, _ := args["action"].(string)
	if action == "" {
		action = "post"
	}

	switch action {
	case "save_session":
		return t.saveSession(ctx)
	case "post":
		msg, _ := args["message"].(string)
		if strings.TrimSpace(msg) == "" {
			return ErrorResult("'message' is required for action 'post' and must not be empty")
		}
		return t.post(ctx, msg)
	default:
		return ErrorResult(fmt.Sprintf("unknown facebook action: %q — valid: post, save_session", action))
	}
}

// sessionFile returns the path to the Facebook browser session file.
func (t *FacebookTool) sessionFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ottoclaw", "browser_sessions", "facebook.json")
}

// saveSession opens a headed browser for the user to login to Facebook manually.
// The session is saved automatically when the browser closes.
func (t *FacebookTool) saveSession(ctx context.Context) *ToolResult {
	browser := &DesktopBrowserTool{}
	result := browser.Execute(ctx, map[string]any{
		"session_id": "facebook",
		"headless":   false,
		"steps": []map[string]any{
			{"action": "navigate", "url": "https://www.facebook.com"},
			// Wait up to 3 minutes for the user to login manually
			{"action": "wait_for", "selector": "[aria-label='สร้างโพสต์'],[aria-label='Create post'],[data-pagelet='FeedUnit_0']", "timeout_ms": 180000},
			{"action": "screenshot", "path": "/tmp/fb_session_ok.png"},
		},
	})

	if result.IsError {
		return ErrorResult("❌ Session save failed: " + result.ForLLM +
			"\nMake sure Playwright is installed: pip install playwright && playwright install chromium")
	}

	return UserResult("✅ Facebook session saved to " + t.sessionFile() + "\n" +
		"You are now logged in. Future 'post' actions will use this session automatically.\n" +
		result.ForLLM)
}

// post publishes a text message to the user's Facebook feed.
func (t *FacebookTool) post(ctx context.Context, message string) *ToolResult {
	sessFile := t.sessionFile()
	if _, err := os.Stat(sessFile); os.IsNotExist(err) {
		return ErrorResult("❌ Facebook session not found.\n" +
			"Please run the facebook tool with action='save_session' first to login.\n" +
			"Session file expected at: " + sessFile)
	}

	browser := &DesktopBrowserTool{}

	// Human-like delay helper — use JS sleep via small wait steps
	steps := []map[string]any{
		// Open Facebook feed
		{"action": "navigate", "url": "https://www.facebook.com"},

		// Wait for the post composer to appear
		{
			"action":     "wait_for",
			"selector":   "[aria-label='สร้างโพสต์'],[aria-label='Create post'],[data-testid='status-attachment-mentions-input'],text=\"คุณคิดอะไรอยู่\",text=\"What's on your mind?\"",
			"timeout_ms": 20000,
		},

		// Click the "What's on your mind?" / "คุณคิดอะไรอยู่" box
		{
			"action":   "click",
			"selector": "[aria-label='สร้างโพสต์'],[aria-label='Create post'],text=\"คุณคิดอะไรอยู่\",text=\"What's on your mind?\"",
		},

		// Wait for the composer modal to open
		{
			"action":     "wait_for",
			"selector":   "[role='dialog'] [contenteditable='true'],[data-testid='status-attachment-mentions-input']",
			"timeout_ms": 10000,
		},

		// Type message character by character via JS to simulate human typing
		{
			"action": "evaluate_js",
			"js": fmt.Sprintf(`
				(function() {
					var el = document.querySelector("[role='dialog'] [contenteditable='true']") ||
					         document.querySelector("[data-testid='status-attachment-mentions-input']");
					if (!el) return 'no_composer_found';
					el.focus();
					return 'focused';
				})()
			`),
		},
		{
			"action": "type_text",
			"text":   message,
		},

		// Short pause before posting
		{"action": "evaluate_js", "js": "new Promise(r => setTimeout(r, 2000))"},

		// Click the Post button
		{
			"action":   "click",
			"selector": "[aria-label='โพสต์'],[aria-label='Post']",
		},

		// Wait for post to be submitted
		{"action": "evaluate_js", "js": fmt.Sprintf("new Promise(r => setTimeout(r, %d))", int(5*time.Second/time.Millisecond))},

		// Screenshot for confirmation
		{"action": "screenshot", "path": "/tmp/fb_posted.png"},
	}

	result := browser.Execute(ctx, map[string]any{
		"session_id":        "facebook",
		"headless":          true,
		"continue_on_error": false,
		"steps":             steps,
	})

	if result.IsError {
		return ErrorResult("❌ Facebook post failed:\n" + result.ForLLM +
			"\n\nTip: If the session expired, run facebook tool with action='save_session' to re-login.")
	}

	// Check that screenshot step exists (confirms browser completed successfully)
	if strings.Contains(result.ForLLM, "screenshot") {
		return UserResult(fmt.Sprintf(
			"✅ Facebook post published successfully!\n"+
				"Message: %q\n"+
				"Screenshot: /tmp/fb_posted.png\n\n"+
				"%s", message, result.ForLLM))
	}

	return UserResult("⚠️ Post may have succeeded but could not confirm.\n" + result.ForLLM)
}
