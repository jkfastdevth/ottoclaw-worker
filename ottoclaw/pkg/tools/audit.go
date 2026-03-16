package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// AuditClient handles sending tool execution logs to the Master.
type AuditClient struct {
	client *siamClient
	nodeID  string
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
	nodeID := os.Getenv("NODE_ID")
	if nodeID == "" {
		nodeID = "unknown-node"
	}
	
	globalAuditClient = &AuditClient{
		client: newSiamClient(masterURL, apiKey),
		nodeID: nodeID,
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
		"node_id":   a.nodeID,
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

func (a *AuditClient) ReportUsage(ctx context.Context, tokens int) {
	if a == nil || a.client == nil || tokens <= 0 {
		return
	}

	agentID := ToolAgentID(ctx)
	if agentID == "" {
		return
	}

	payload := map[string]any{
		"tokens": tokens,
	}

	// Non-blocking report to avoid slowing down the agent
	go func() {
		_, err := a.client.post(fmt.Sprintf("/api/agent/v1/agents/%s/usage", agentID), payload)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️ Failed to report usage: %v\n", err)
		}
	}()
}

// RequestApproval sends an approval request to the Master
func (a *AuditClient) RequestApproval(ctx context.Context, toolName string, input string) (string, error) {
	if a == nil || a.client == nil {
		return "", nil // Development mode
	}

	agentID := ToolAgentID(ctx)
	if agentID == "" {
		agentID = "unknown-worker"
	}

	payload := map[string]any{
		"agent_id":  agentID,
		"tool_name": toolName,
		"input":     input,
	}

	data, err := a.client.post("/api/agent/v1/approvals", payload)
	if err != nil {
		return "", err
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}

	return result.ID, nil
}

// GetApprovalStatus checks the status of an approval request
func (a *AuditClient) GetApprovalStatus(ctx context.Context, approvalID string) (string, error) {
	if a == nil || a.client == nil {
		return "approved", nil // Development mode
	}

	data, err := a.client.get(fmt.Sprintf("/api/agent/v1/approvals/%s", approvalID))
	if err != nil {
		return "", err
	}

	var result struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}

	return result.Status, nil
}

// WaitApproval polls for approval status until it is no longer pending
func (a *AuditClient) WaitApproval(ctx context.Context, toolName string, input string) (bool, error) {
	approvalID, err := a.RequestApproval(ctx, toolName, input)
	if err != nil {
		return false, fmt.Errorf("failed to request approval: %w", err)
	}

	if approvalID == "" {
		return true, nil // Dev mode or skipped
	}

	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(5 * time.Second):
			status, err := a.GetApprovalStatus(ctx, approvalID)
			if err != nil {
				return false, fmt.Errorf("failed to check approval status: %w", err)
			}

			switch status {
			case "approved":
				return true, nil
			case "rejected":
				return false, nil
			}
			// continue polling for "pending"
		}
	}
}
