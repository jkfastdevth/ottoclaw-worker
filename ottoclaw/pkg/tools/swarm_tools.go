package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/sipeed/ottoclaw/pkg/utils"
)

type SwarmNegotiateTool struct {
	MasterURL string
	APIKey    string
	AgentID   string
}

func NewSwarmNegotiateTool(masterURL, apiKey, agentID string) *SwarmNegotiateTool {
	if masterURL == "" {
		masterURL = os.Getenv("MASTER_API_URL")
		if masterURL == "" {
			masterURL = "http://master:8080"
		}
	}
	if apiKey == "" {
		apiKey = os.Getenv("MASTER_API_KEY")
	}
	return &SwarmNegotiateTool{
		MasterURL: masterURL,
		APIKey:    apiKey,
		AgentID:   agentID,
	}
}

func (t *SwarmNegotiateTool) Name() string { return "siam_negotiate" }
func (t *SwarmNegotiateTool) Description() string { return "Propose a negotiation or deal to another agent. Useful for task swapping or acquiring resources from peers." }
func (t *SwarmNegotiateTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"target_agent": map[string]any{
				"type":        "string",
				"description": "The ID or Name of the agent you want to negotiate with",
			},
			"proposal": map[string]any{
				"type":        "string",
				"description": "The details of the deal you are proposing (e.g., 'I will handle X if you handle Y')",
			},
		},
		"required": []string{"target_agent", "proposal"},
	}
}

func (t *SwarmNegotiateTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	targetAgent, ok := args["target_agent"].(string)
	if !ok || targetAgent == "" {
		return ErrorResult("missing or invalid target_agent")
	}
	proposal, ok := args["proposal"].(string)
	if !ok || proposal == "" {
		return ErrorResult("missing or invalid proposal")
	}

	payload := map[string]string{
		"source_agent": t.AgentID,
		"target_agent": targetAgent,
		"proposal":     proposal,
	}
	body, _ := json.Marshal(payload)

	apiURL := fmt.Sprintf("%s/api/agent/v1/swarm/negotiate", t.MasterURL)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(body))
	if err != nil {
		return ErrorResult(err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	if t.APIKey != "" {
		req.Header.Set("X-API-Key", t.APIKey)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	proxy := utils.GetEffectiveProxy("")
	if proxy != "" {
		if parsed, err := url.Parse(proxy); err == nil {
			client.Transport = &http.Transport{
				Proxy: http.ProxyURL(parsed),
			}
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to reach swarm orchestrator: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return ErrorResult(fmt.Sprintf("swarm orchestrator rejected payload (status %d)", resp.StatusCode))
	}

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	id, _ := result["id"].(string)
	return NewToolResult(fmt.Sprintf("Negotiation proposal sent to %s. Reference ID: %s. Wait for their response.", targetAgent, id))
}

type SwarmVoteTool struct {
	MasterURL string
	APIKey    string
	AgentID   string
}

func NewSwarmVoteTool(masterURL, apiKey, agentID string) *SwarmVoteTool {
	if masterURL == "" {
		masterURL = os.Getenv("MASTER_API_URL")
		if masterURL == "" {
			masterURL = "http://master:8080"
		}
	}
	if apiKey == "" {
		apiKey = os.Getenv("MASTER_API_KEY")
	}
	return &SwarmVoteTool{
		MasterURL: masterURL,
		APIKey:    apiKey,
		AgentID:   agentID,
	}
}

func (t *SwarmVoteTool) Name() string { return "siam_cast_vote" }
func (t *SwarmVoteTool) Description() string { return "Cast a vote on a pending consensus proposal. Requires 'Yes' or 'No'." }
func (t *SwarmVoteTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"consensus_id": map[string]any{
				"type":        "string",
				"description": "The ID of the consensus proposal you are voting on",
			},
			"vote": map[string]any{
				"type":        "string",
				"description": "Your vote ('Yes' or 'No')",
			},
		},
		"required": []string{"consensus_id", "vote"},
	}
}

func (t *SwarmVoteTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	consensusID, ok := args["consensus_id"].(string)
	if !ok || consensusID == "" {
		return ErrorResult("missing or invalid consensus_id")
	}
	vote, ok := args["vote"].(string)
	if !ok || (vote != "Yes" && vote != "No") {
		return ErrorResult("vote must be 'Yes' or 'No'")
	}

	payload := map[string]string{
		"agent_id": t.AgentID,
		"vote":     vote,
	}
	body, _ := json.Marshal(payload)

	apiURL := fmt.Sprintf("%s/api/agent/v1/swarm/consensus/%s/vote", t.MasterURL, consensusID)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(body))
	if err != nil {
		return ErrorResult(err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	if t.APIKey != "" {
		req.Header.Set("X-API-Key", t.APIKey)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	proxy := utils.GetEffectiveProxy("")
	if proxy != "" {
		if parsed, err := url.Parse(proxy); err == nil {
			client.Transport = &http.Transport{
				Proxy: http.ProxyURL(parsed),
			}
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to cast vote: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return ErrorResult(fmt.Sprintf("vote rejected (status %d): %s", resp.StatusCode, string(b)))
	}

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	status, _ := result["current_status"].(string)
	return NewToolResult(fmt.Sprintf("Vote cast successfully. Consensus status: %s", status))
}
