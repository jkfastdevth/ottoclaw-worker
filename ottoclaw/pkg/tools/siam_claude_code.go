package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// SiamClaudeCodeTool lets agents delegate code tasks to the Claude Code Worker.
type SiamClaudeCodeTool struct{}

func (t *SiamClaudeCodeTool) Name() string { return "siam_claude_code" }
func (t *SiamClaudeCodeTool) Description() string {
	return "Send code fix/improvement/analysis requests to the Claude Code Worker. Supports: bug fixes, new features, refactoring, system analysis, business plan design."
}

func (t *SiamClaudeCodeTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"task": map[string]any{
				"type":        "string",
				"description": "Description of the code task for Claude Code to perform",
			},
			"files": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Relevant files to pre-load (optional), e.g. ['master/api/channels.go']",
			},
			"auto_commit": map[string]any{
				"type":        "boolean",
				"description": "Commit automatically without waiting for approval (default: false)",
			},
			"notify_target": map[string]any{
				"type":        "string",
				"description": "Channel target for result notification, e.g. channel:telegram:1234567",
			},
		},
		"required": []string{"task"},
	}
}

func (t *SiamClaudeCodeTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	masterURL := os.Getenv("MASTER_URL")
	if masterURL == "" {
		masterURL = "http://master:8080"
	}
	apiKey := os.Getenv("MASTER_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("SIAM_API_KEY")
	}

	payload := map[string]any{
		"task": fmt.Sprintf("%v", args["task"]),
	}
	if files, ok := args["files"]; ok {
		payload["files"] = files
	}
	if ac, ok := args["auto_commit"]; ok {
		payload["auto_commit"] = ac
	}
	if nt, ok := args["notify_target"]; ok {
		payload["notify_target"] = nt
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", masterURL+"/api/agent/v1/claude-worker/task", bytes.NewReader(body))
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to create request: %v", err))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to call claude-worker: %v", err))
	}
	defer resp.Body.Close()

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode != 200 {
		return ErrorResult(fmt.Sprintf("Claude Worker error: %v", result["error"]))
	}

	taskID := fmt.Sprintf("%v", result["task_id"])
	shortID := taskID
	if len(taskID) > 8 {
		shortID = taskID[:8]
	}

	// Poll for completion (up to 3 minutes)
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		time.Sleep(8 * time.Second)

		statusReq, _ := http.NewRequestWithContext(ctx, "GET", masterURL+"/api/agent/v1/claude-worker/tasks/"+taskID, nil)
		statusReq.Header.Set("X-API-Key", apiKey)
		statusResp, err := client.Do(statusReq)
		if err != nil {
			continue
		}
		var statusResult map[string]any
		json.NewDecoder(statusResp.Body).Decode(&statusResult)
		statusResp.Body.Close()

		status := fmt.Sprintf("%v", statusResult["status"])
		if status == "done" || status == "approved" || status == "failed" || status == "rejected" {
			resultText := fmt.Sprintf("%v", statusResult["result"])
			diff := fmt.Sprintf("%v", statusResult["diff"])
			if diff == "<nil>" || diff == "" {
				diff = "(no changes)"
			} else if len(diff) > 600 {
				diff = diff[:600] + "\n... [truncated]"
			}

			if status == "failed" {
				return ErrorResult(fmt.Sprintf("Task %s failed:\n%v", shortID, statusResult["error"]))
			}
			if status == "approved" {
				return UserResult(fmt.Sprintf("Task %s completed & committed:\n%s", shortID, resultText))
			}

			// done — needs approval
			notifyTarget := ""
			if nt, ok := args["notify_target"]; ok {
				notifyTarget = fmt.Sprintf("%v", nt)
			}
			if notifyTarget == "" {
				return UserResult(fmt.Sprintf("Task %s done (awaiting approval):\n%s\n\nDiff:\n```\n%s\n```\n\nApprove: POST /api/agent/v1/claude-worker/tasks/%s/approve", shortID, resultText, diff, taskID))
			}
			return UserResult(fmt.Sprintf("Task %s done:\n%s\n\nApproval notification sent to channel.", shortID, resultText))
		}

		// Check for context cancellation
		select {
		case <-ctx.Done():
			return NewToolResult(fmt.Sprintf("Task %s is still running (context cancelled). Check status at /api/agent/v1/claude-worker/tasks/%s", shortID, taskID))
		default:
		}
	}

	return NewToolResult(fmt.Sprintf("Task %s is taking longer than expected. Task ID: %s", shortID, taskID))
}
