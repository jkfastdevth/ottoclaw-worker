package tools

import (
	"context"
	"fmt"
	"os"
)

// AuditClient handles sending tool execution logs to the Master.
type AuditClient struct {
	client *siamClient
}

var globalAuditClient *AuditClient

func GetAuditClient() *AuditClient {
	if globalAuditClient != nil {
		return globalAuditClient
	}

	masterURL := os.Getenv("SIAM_MASTER_URL")
	if masterURL == "" {
		masterURL = os.Getenv("MASTER_API_URL")
	}
	if masterURL == "" {
		return nil
	}
	apiKey := os.Getenv("SIAM_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("MASTER_API_KEY")
	}
	
	globalAuditClient = &AuditClient{
		client: newSiamClient(masterURL, apiKey),
	}
	return globalAuditClient
}

func (a *AuditClient) Log(ctx context.Context, toolName, input, output, status string) {
	if a == nil || a.client == nil {
		return
	}

	agentID := ToolAgentID(ctx)
	if agentID == "" {
		agentID = "unknown-worker"
	}

	payload := map[string]any{
		"agent_id":  agentID,
		"tool_name": toolName,
		"input":     input,
		"output":    output,
		"status":    status,
	}

	// Non-blocking log to avoid slowing down the agent
	go func() {
		_, err := a.client.post("/api/agent/v1/audit/log", payload)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️ Failed to send audit log: %v\n", err)
		}
	}()
}
