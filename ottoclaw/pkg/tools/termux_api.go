package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// TermuxAPITool provides access to Android hardware features via the termux-api package.
type TermuxAPITool struct{}

func NewTermuxAPITool() *TermuxAPITool {
	return &TermuxAPITool{}
}

func (t *TermuxAPITool) Name() string {
	return "termux_api"
}

func (t *TermuxAPITool) Description() string {
	return "Access Android hardware features via termux-api. " +
		"Supported actions: vibrate, toast, battery-status, location, camera-info, camera-photo, clipboard-get, clipboard-set. " +
		"Requires the termux-api package to be installed on the device."
}

func (t *TermuxAPITool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type": "string",
				"enum": []string{
					"vibrate", 
					"toast", 
					"battery-status", 
					"location", 
					"camera-info", 
					"camera-photo", 
					"clipboard-get", 
					"clipboard-set",
				},
				"description": "The termux-api action to execute.",
			},
			"args": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Optional arguments for the action. For vibrate: ['-d', '1000'] (duration in ms). For toast: ['-s', 'Message'] or just ['Message']. For clipboard-set: ['Text to copy']. For camera-photo: ['-c', '0', 'file.jpg'].",
			},
		},
		"required": []string{"action"},
	}
}

func (t *TermuxAPITool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	action, ok := args["action"].(string)
	if !ok || action == "" {
		return ErrorResult("action is required")
	}

	command := "termux-" + action

	// Check if termux-api is installed
	if _, err := exec.LookPath(command); err != nil {
		return ErrorResult(fmt.Sprintf("%s not found. Is termux-api installed? Run 'pkg install termux-api'", command))
	}

	cmdArgs := []string{}
	if rawArgs, ok := args["args"].([]any); ok {
		for _, arg := range rawArgs {
			cmdArgs = append(cmdArgs, fmt.Sprint(arg))
		}
	}

	cmd := exec.CommandContext(ctx, command, cmdArgs...)
	
	// Ensure we run in a clean environment to avoid issues
	cmd.Env = append(cmd.Environ(), "LD_PRELOAD=")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
		return ErrorResult(fmt.Sprintf("termux-api failed: %s", errMsg))
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		output = fmt.Sprintf("Action '%s' completed successfully.", action)
	}

	return &ToolResult{
		ForLLM:  output,
		ForUser: output,
	}
}
