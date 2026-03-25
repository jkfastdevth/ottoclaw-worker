package tools

import (
	"context"
	"fmt"
	"strings"
)

// SiamRequestApprovalTool — request human approval for an action.
type SiamRequestApprovalTool struct {
	client *siamClient
}

func (t *SiamRequestApprovalTool) Name() string { return "siam_request_approval" }
func (t *SiamRequestApprovalTool) Description() string {
	return "Request human approval for a high-risk action. This creates a mission in 'pending_approval' state. You should use siam_get_mission to poll for approval status before proceeding."
}

func (t *SiamRequestApprovalTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"agent_id": map[string]any{
				"type":        "string",
				"description": "Your Agent ID (e.g. 'auric-spark').",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "What do you want to do? (e.g. 'I want to restart the database server to fix a memory leak').",
			},
		},
		"required": []string{"agent_id", "description"},
	}
}

func (t *SiamRequestApprovalTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	agentIDRaw, _ := args["agent_id"].(string)
	description, _ := args["description"].(string)

	if strings.TrimSpace(agentIDRaw) == "" || strings.TrimSpace(description) == "" {
		return ErrorResult("siam_request_approval: agent_id and description are required")
	}

	agentID := strings.ToLower(strings.TrimSpace(agentIDRaw))
	agentID = strings.ReplaceAll(agentID, " ", "-")

	payload := map[string]any{
		"agent_id":          agentID,
		"description":       description,
		"requires_approval": true,
		"notify_target":     ToolReplyTo(ctx),
	}

	data, err := t.client.post("/api/agent/v1/missions", payload)
	if err != nil {
		return ErrorResult(fmt.Sprintf("siam_request_approval failed: %v", err))
	}
	return UserResult(string(data))
}

// SiamGetMissionTool — get a mission's status.
type SiamGetMissionTool struct {
	client *siamClient
}

func (t *SiamGetMissionTool) Name() string { return "siam_get_mission" }
func (t *SiamGetMissionTool) Description() string {
	return "Check the status of a specific mission by its ID. Useful for tracking human approval status (pending_approval -> pending/failed)."
}

func (t *SiamGetMissionTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"mission_id": map[string]any{
				"type":        "string",
				"description": "The UUID of the mission to check.",
			},
		},
		"required": []string{"mission_id"},
	}
}

func (t *SiamGetMissionTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	missionID, _ := args["mission_id"].(string)
	if strings.TrimSpace(missionID) == "" {
		return ErrorResult("siam_get_mission: mission_id is required")
	}

	data, err := t.client.get("/api/agent/v1/missions/detail/" + missionID)
	if err != nil {
		return ErrorResult(fmt.Sprintf("siam_get_mission failed: %v", err))
	}
	return UserResult(string(data))
}
