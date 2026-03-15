package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/sipeed/ottoclaw/pkg/logger"
)

type BroadcastTool struct {
	MasterURL string
	APIKey    string
}

func NewBroadcastTool() *BroadcastTool {
	masterURL := os.Getenv("MASTER_API_URL")
	if masterURL == "" {
		masterURL = "http://master:8080"
	}
	apiKey := os.Getenv("MASTER_API_KEY")

	return &BroadcastTool{
		MasterURL: masterURL,
		APIKey:    apiKey,
	}
}

func (t *BroadcastTool) Name() string { return "siam_broadcast" }
func (t *BroadcastTool) Description() string {
	return "Send a message to external communication channels (telegram, slack, or all)."
}

func (t *BroadcastTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"message": map[string]any{
				"type":        "string",
				"description": "The message content to broadcast.",
			},
			"channel": map[string]any{
				"type":        "string",
				"description": "The target channel (telegram, slack, or all).",
				"enum":        []string{"telegram", "slack", "all"},
			},
		},
		"required": []string{"message", "channel"},
	}
}

func (t *BroadcastTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	message, ok1 := args["message"].(string)
	channel, ok2 := args["channel"].(string)
	if !ok1 || !ok2 {
		return ErrorResult("invalid arguments")
	}

	payload := map[string]string{
		"message": message,
		"channel": channel,
	}
	body, _ := json.Marshal(payload)

	url := fmt.Sprintf("%s/api/agent/v1/bridge/broadcast", t.MasterURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to create request: %v", err))
	}

	req.Header.Set("Content-Type", "application/json")
	if t.APIKey != "" {
		req.Header.Set("X-API-Key", t.APIKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to call bridge: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errData map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errData)
		return ErrorResult(fmt.Sprintf("bridge error: %v", errData["error"]))
	}

	logger.InfoCF("tools", "Broadcast successful", map[string]any{
		"channel": channel,
		"message": message,
	})

	return NewToolResult(fmt.Sprintf("Successfully broadcasted message to %s", channel))
}
