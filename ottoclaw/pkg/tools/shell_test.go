package tools

import (
	"context"
	"os"
	"testing"
)

func TestExecToolSafety(t *testing.T) {
	wd, _ := os.Getwd()
	tool, _ := NewExecTool(wd, true)

	// Test restricted command
	res := tool.Execute(context.Background(), map[string]any{"command": "rm -rf /"})
	if !res.IsError {
		t.Error("Command 'rm -rf /' should be blocked")
	}

	// Test path traversal
	res = tool.Execute(context.Background(), map[string]any{"command": "ls ../"})
	if !res.IsError {
		t.Error("Command 'ls ../' should be blocked")
	}

	// Test absolute path outside workspace
	res = tool.Execute(context.Background(), map[string]any{"command": "cat /etc/passwd"})
	if !res.IsError {
		t.Error("Command 'cat /etc/passwd' should be blocked")
	}
}

func TestExecToolStrictMode(t *testing.T) {
	os.Setenv("OTTOCLAW_SHELL_STRICT", "true")
	os.Setenv("OTTOCLAW_SHELL_ALLOWLIST", "ls,echo")
	defer os.Unsetenv("OTTOCLAW_SHELL_STRICT")
	defer os.Unsetenv("OTTOCLAW_SHELL_ALLOWLIST")

	wd, _ := os.Getwd()
	tool, _ := NewExecTool(wd, false)

	// Test allowed command
	res := tool.Execute(context.Background(), map[string]any{"command": "echo hello"})
	if res.IsError {
		t.Errorf("Command 'echo hello' should be allowed, got error: %v", res.ForLLM)
	}

	// Test disallowed command
	res = tool.Execute(context.Background(), map[string]any{"command": "whoami"})
	if !res.IsError {
		t.Error("Command 'whoami' should be blocked in strict mode")
	}
}
