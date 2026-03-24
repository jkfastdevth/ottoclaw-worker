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
	"strings"
	"time"

	"github.com/sipeed/ottoclaw/pkg/utils"
)

// siamClient is a shared HTTP helper for Siam-Synapse Master API calls.
type siamClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func newSiamClient(baseURL, apiKey string) *siamClient {
	client := &http.Client{Timeout: 15 * time.Second}
	proxy := utils.GetEffectiveProxy("")
	if proxy != "" {
		if parsed, err := url.Parse(proxy); err == nil {
			client.Transport = &http.Transport{
				Proxy: http.ProxyURL(parsed),
			}
		}
	}
	return &siamClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    client,
	}
}

func (c *siamClient) do(method, path string, body any) ([]byte, int, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.baseURL+path, r)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return data, resp.StatusCode, nil
}

func (c *siamClient) get(path string) ([]byte, error) {
	data, _, err := c.do("GET", path, nil)
	return data, err
}

func (c *siamClient) post(path string, body any) ([]byte, error) {
	data, _, err := c.do("POST", path, body)
	return data, err
}

func (c *siamClient) delete(path string) ([]byte, error) {
	data, _, err := c.do("DELETE", path, nil)
	return data, err
}

// AuditAction logs a tool execution to the Master.
func (c *siamClient) AuditAction(agentID, nodeID, toolName, input, output, status string) {
	payload := map[string]any{
		"agent_id":  agentID,
		"node_id":   nodeID,
		"tool_name": toolName,
		"input":     input,
		"output":    output,
		"status":    status,
	}
	_, _, err := c.do("POST", "/api/agent/v1/audit/log", payload)
	if err != nil {
		fmt.Printf("⚠️  Failed to log audit for %s: %v\n", toolName, err)
	}
}

// NewSiamToolset creates all Siam-Synapse tools, reading SIAM_MASTER_URL
// and SIAM_API_KEY (or MASTER_API_URL / MASTER_API_KEY) from the environment.
func NewSiamToolset(masterURL, apiKey string) ([]Tool, AuditLogger) {
	client := newSiamClient(masterURL, apiKey)
	toolset := []Tool{
		&SiamGetMetricsTool{client: client},
		&SiamGetAgentsTool{client: client},
		&SiamGetNodesTool{client: client},
		&SiamGetMessagesTool{client: client},
		&SiamSpawnAgentTool{client: client},
		&SiamTerminateAgentTool{client: client},
		&SiamGetSkillsTool{client: client},
		&SiamFindAgentsTool{client: client},
		&SiamScaleTool{client: client},
		&SiamGetJobsTool{client: client},
		&SiamSubmitJobTool{client: client},
		&SiamRunCommandTool{client: client},
		&SiamSendMessageTool{client: client},
		&SiamDelegateMissionTool{client: client},
		&SiamStoreMemoryTool{client: client},
		&SiamSearchMemoryTool{client: client},
		&SiamRequestApprovalTool{client: client},
		&SiamGetMissionTool{client: client},
		&SiamPromoteAgentTool{client: client},
		&SiamPromotionRitualTool{client: client},
		&SiamBroadcastUpdateTool{client: client},
		&SiamOpenBrowserTool{client: client},
		&SiamSendEmailTool{},
		&SiamListCalendarTool{},
		&SiamCreateCalendarEventTool{},
		&SiamDriveUploadTool{},
		&SiamDriveSearchTool{},
		&SiamDriveDownloadTool{},
		&SiamReadEmailsTool{},
	}
	return toolset, client
}

// SiamDelegateMissionTool — delegate a persistent task to another agent.
type SiamDelegateMissionTool struct{ client *siamClient }

func (t *SiamDelegateMissionTool) Name() string { return "siam_delegate_mission" }
func (t *SiamDelegateMissionTool) Description() string {
	return "Delegate a persistent, long-running mission to another Siam-Synapse sub-agent. This is more reliable than siam_send_message for complex tasks as it persists across restarts."
}
func (t *SiamDelegateMissionTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"agent_id": map[string]any{
				"type":        "string",
				"description": "The target Agent ID / Soul Name (e.g. 'kaidos').",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Detailed mission directive for the agent (e.g. 'Research Bitcoin trends for the next 24h').",
			},
			"parent_id": map[string]any{
				"type":        "string",
				"description": "Optional: ID of the current mission you are working on, to link this new task as a sub-mission.",
			},
			"requires_approval": map[string]any{
				"type":        "boolean",
				"description": "Optional: If true, the mission will require human approval before starting.",
			},
		},
		"required": []string{"agent_id", "description"},
	}
}
func (t *SiamDelegateMissionTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	agentIDRaw, _ := args["agent_id"].(string)
	description, _ := args["description"].(string)
	parentID, _ := args["parent_id"].(string)

	if strings.TrimSpace(agentIDRaw) == "" || strings.TrimSpace(description) == "" {
		return ErrorResult("siam_delegate_mission: agent_id and description are required")
	}

	// Normalize target ID
	agentID := strings.ToLower(strings.TrimSpace(agentIDRaw))
	agentID = strings.ReplaceAll(agentID, " ", "-")

	payload := map[string]any{
		"agent_id":    agentID,
		"description": description,
	}
	if parentID != "" {
		payload["parent_id"] = parentID
	}
	if reqApp, ok := args["requires_approval"].(bool); ok && reqApp {
		payload["requires_approval"] = true
	}

	data, err := t.client.post("/api/agent/v1/missions", payload)
	if err != nil {
		return ErrorResult(fmt.Sprintf("siam_delegate_mission failed: %v", err))
	}
	return UserResult(string(data))
}

// SiamStoreMemoryTool — store fact in Akashic Library.
type SiamStoreMemoryTool struct{ client *siamClient }

func (t *SiamStoreMemoryTool) Name() string { return "siam_store_memory" }
func (t *SiamStoreMemoryTool) Description() string {
	return "Store an important fact or observation in the shared Akashic Library (Global Intelligence). This knowledge becomes accessible to all agents in the network."
}
func (t *SiamStoreMemoryTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"fact": map[string]any{
				"type":        "string",
				"description": "The specific information or fact to store (e.g. 'Bitcoin reached $100k at 10:45 AM UTC').",
			},
			"confidence": map[string]any{
				"type":        "number",
				"description": "Confidence level (0.0 to 1.0). Default is 1.0.",
			},
			"tags": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "string",
				},
				"description": "Keywords for indexing (e.g. ['crypto', 'price', 'alert']).",
			},
		},
		"required": []string{"fact"},
	}
}
func (t *SiamStoreMemoryTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	fact, _ := args["fact"].(string)
	confidence, _ := args["confidence"].(float64)
	tagsRaw, _ := args["tags"].([]any)

	if strings.TrimSpace(fact) == "" {
		return ErrorResult("siam_store_memory: fact is required")
	}

	tags := make([]string, 0, len(tagsRaw))
	for _, tr := range tagsRaw {
		if s, ok := tr.(string); ok {
			tags = append(tags, s)
		}
	}

	payload := map[string]any{
		"fact":         fact,
		"confidence":   confidence,
		"source_agent": os.Getenv("AGENT_NAME"),
		"tags":         tags,
	}

	data, err := t.client.post("/api/agent/v1/knowledge", payload)
	if err != nil {
		return ErrorResult(fmt.Sprintf("siam_store_memory failed: %v", err))
	}
	return UserResult(string(data))
}

// SiamSearchMemoryTool — search the shared library.
type SiamSearchMemoryTool struct{ client *siamClient }

func (t *SiamSearchMemoryTool) Name() string { return "siam_search_memory" }
func (t *SiamSearchMemoryTool) Description() string {
	return "Search the shared Akashic Library for facts, research, or observations gathered by other agents in the network."
}
func (t *SiamSearchMemoryTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search query or keyword (e.g. 'Bitcoin analysis').",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Max results to return. Default is 5.",
			},
		},
		"required": []string{"query"},
	}
}
func (t *SiamSearchMemoryTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	query, _ := args["query"].(string)
	limit, _ := args["limit"].(float64)

	if strings.TrimSpace(query) == "" {
		return ErrorResult("siam_search_memory: query is required")
	}

	if limit == 0 {
		limit = 5
	}

	results, err := t.Search(context.Background(), query, int(limit))
	if err != nil {
		return ErrorResult(fmt.Sprintf("siam_search_memory failed: %v", err))
	}
	return UserResult(results)
}

func (t *SiamSearchMemoryTool) Search(ctx context.Context, query string, limit int) (string, error) {
	path := fmt.Sprintf("/api/agent/v1/knowledge/search?q=%s&limit=%d", query, limit)
	data, err := t.client.get(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SiamGetMetricsTool — get system CPU / node metrics.
type SiamGetMetricsTool struct{ client *siamClient }

func (t *SiamGetMetricsTool) Name() string { return "siam_get_metrics" }
func (t *SiamGetMetricsTool) Description() string {
	return "Get current Siam-Synapse system metrics: CPU usage, active nodes, worker count, and scaling mode."
}
func (t *SiamGetMetricsTool) Parameters() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (t *SiamGetMetricsTool) Execute(_ context.Context, _ map[string]any) *ToolResult {
	data, err := t.client.get("/api/agent/v1/metrics")
	if err != nil {
		return ErrorResult(fmt.Sprintf("siam_get_metrics: %v", err))
	}
	return UserResult(string(data))
}

// SiamGetAgentsTool — list running sub-agents.
type SiamGetAgentsTool struct{ client *siamClient }

func (t *SiamGetAgentsTool) Name() string { return "siam_get_agents" }
func (t *SiamGetAgentsTool) Description() string {
	return "List all currently running sub-agents managed by Siam-Synapse Master. Returns agent IDs, missions, statuses, and node IPs."
}
func (t *SiamGetAgentsTool) Parameters() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (t *SiamGetAgentsTool) Execute(_ context.Context, _ map[string]any) *ToolResult {
	data, err := t.client.get("/api/agent/v1/agents")
	if err != nil {
		return ErrorResult(fmt.Sprintf("siam_get_agents: %v", err))
	}
	return UserResult(string(data))
}

// SiamSpawnAgentTool — deploy a new Docker worker agent.
type SiamSpawnAgentTool struct{ client *siamClient }

func (t *SiamSpawnAgentTool) Name() string { return "siam_spawn_agent" }
func (t *SiamSpawnAgentTool) Description() string {
	return "Spawn a new Docker-based sub-agent on the Siam-Synapse network. Use siam_get_skills first to find the correct agent_image for the required skill."
}
func (t *SiamSpawnAgentTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"agent_id": map[string]any{
				"type":        "string",
				"description": "Unique name for the new agent (e.g. 'trader-paxg-01')",
			},
			"mission": map[string]any{
				"type":        "string",
				"description": "Short, focused task description for the agent",
			},
			"node_ip": map[string]any{
				"type":        "string",
				"description": "Target node IP or 'local'. Default: local",
			},
			"agent_image": map[string]any{
				"type":        "string",
				"description": "Docker image to use (from siam_get_skills results)",
			},
		},
		"required": []string{"agent_id", "mission"},
	}
}
func (t *SiamSpawnAgentTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	agentID, _ := args["agent_id"].(string)
	mission, _ := args["mission"].(string)
	nodeIP, _ := args["node_ip"].(string)
	agentImage, _ := args["agent_image"].(string)
	if strings.TrimSpace(agentID) == "" || strings.TrimSpace(mission) == "" {
		return ErrorResult("siam_spawn_agent: agent_id and mission are required")
	}
	if nodeIP == "" {
		nodeIP = "local"
	}
	payload := map[string]any{
		"agent_id":    agentID,
		"mission":     mission,
		"node_ip":     nodeIP,
		"agent_image": agentImage,
	}
	data, err := t.client.post("/api/agent/v1/agents/spawn", payload)
	if err != nil {
		return ErrorResult(fmt.Sprintf("siam_spawn_agent: %v", err))
	}
	return UserResult(string(data))
}

// SiamTerminateAgentTool — stop and remove a running agent.
type SiamTerminateAgentTool struct{ client *siamClient }

func (t *SiamTerminateAgentTool) Name() string { return "siam_terminate_agent" }
func (t *SiamTerminateAgentTool) Description() string {
	return "Stop and remove a running Siam-Synapse sub-agent container by its agent_id."
}
func (t *SiamTerminateAgentTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"agent_id": map[string]any{
				"type":        "string",
				"description": "ID of the agent to terminate",
			},
		},
		"required": []string{"agent_id"},
	}
}
func (t *SiamTerminateAgentTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	agentID, _ := args["agent_id"].(string)
	if strings.TrimSpace(agentID) == "" {
		return ErrorResult("siam_terminate_agent: agent_id is required")
	}
	data, err := t.client.delete("/api/agent/v1/agents/" + agentID)
	if err != nil {
		return ErrorResult(fmt.Sprintf("siam_terminate_agent: %v", err))
	}
	return UserResult(string(data))
}

// SiamGetSkillsTool — query skill registry.
type SiamGetSkillsTool struct{ client *siamClient }

func (t *SiamGetSkillsTool) Name() string { return "siam_get_skills" }
func (t *SiamGetSkillsTool) Description() string {
	return "Query the Siam-Synapse Skill Registry to find available skills and the Docker images that provide them. Always call this before siam_spawn_agent to find the correct agent_image."
}
func (t *SiamGetSkillsTool) Parameters() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (t *SiamGetSkillsTool) Execute(_ context.Context, _ map[string]any) *ToolResult {
	data, err := t.client.get("/api/agent/v1/skills")
	if err != nil {
		return ErrorResult(fmt.Sprintf("siam_get_skills: %v", err))
	}
	return UserResult(string(data))
}

// SiamScaleTool — scale workers up or down.
type SiamScaleTool struct{ client *siamClient }

func (t *SiamScaleTool) Name() string { return "siam_scale" }
func (t *SiamScaleTool) Description() string {
	return "Manually scale Siam-Synapse workers. action must be 'up' (add worker) or 'down' (remove worker). Use siam_get_metrics first to check CPU and node count."
}
func (t *SiamScaleTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"up", "down"},
				"description": "Scale direction: 'up' to add workers, 'down' to remove idle workers",
			},
		},
		"required": []string{"action"},
	}
}
func (t *SiamScaleTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	action, _ := args["action"].(string)
	if action != "up" && action != "down" {
		return ErrorResult("siam_scale: action must be 'up' or 'down'")
	}
	data, err := t.client.post("/api/agent/v1/scale", map[string]any{"action": action})
	if err != nil {
		return ErrorResult(fmt.Sprintf("siam_scale: %v", err))
	}
	return UserResult(string(data))
}

// SiamGetJobsTool — list all ottoclaw one-shot jobs.
type SiamGetJobsTool struct{ client *siamClient }

func (t *SiamGetJobsTool) Name() string { return "siam_get_jobs" }
func (t *SiamGetJobsTool) Description() string {
	return "List all OttoClaw one-shot jobs with their current status (deployed, running, succeeded, failed). Returns job_id, state, model, output, and timing info."
}
func (t *SiamGetJobsTool) Parameters() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (t *SiamGetJobsTool) Execute(_ context.Context, _ map[string]any) *ToolResult {
	data, err := t.client.get("/api/agent/v1/ottoclaw/jobs")
	if err != nil {
		return ErrorResult(fmt.Sprintf("siam_get_jobs: %v", err))
	}
	return UserResult(string(data))
}

// SiamSubmitJobTool — submit a one-shot ottoclaw job.
type SiamSubmitJobTool struct{ client *siamClient }

func (t *SiamSubmitJobTool) Name() string { return "siam_submit_job" }
func (t *SiamSubmitJobTool) Description() string {
	return "Submit a one-shot OttoClaw job: spins up a fresh container, runs a single LLM message, captures the output, then exits. Great for isolated AI sub-tasks."
}
func (t *SiamSubmitJobTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"message": map[string]any{
				"type":        "string",
				"description": "The message/task to send to the OttoClaw agent in the one-shot job",
			},
			"model_id": map[string]any{
				"type":        "string",
				"description": "Optional model ID override (e.g. 'llama-3.3-70b-versatile'). Leave empty to use system default.",
			},
		},
		"required": []string{"message"},
	}
}
func (t *SiamSubmitJobTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	message, _ := args["message"].(string)
	if strings.TrimSpace(message) == "" {
		return ErrorResult("siam_submit_job: message is required")
	}
	payload := map[string]any{"message": message}
	if modelID, ok := args["model_id"].(string); ok && strings.TrimSpace(modelID) != "" {
		payload["model_id"] = modelID
	}
	data, err := t.client.post("/api/agent/v1/ottoclaw/jobs", payload)
	if err != nil {
		return ErrorResult(fmt.Sprintf("siam_submit_job: %v", err))
	}
	return UserResult(string(data))
}

// SiamRunCommandTool — execute a shell command on a remote node.
type SiamRunCommandTool struct{ client *siamClient }

func (t *SiamRunCommandTool) Name() string { return "siam_run_command" }
func (t *SiamRunCommandTool) Description() string {
	return "Execute a shell command on a specific Siam-Synapse node (e.g. a remote worker connected via gRPC or Tailscale). The node must be online."
}
func (t *SiamRunCommandTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"node_id": map[string]any{
				"type":        "string",
				"description": "The target Node ID (e.g. 'worker-ubuntu-01' or 'remote-node-01'). Use siam_get_metrics to see connected nodes.",
			},
			"command": map[string]any{
				"type":        "string",
				"description": "The shell command to execute (e.g. 'ls -la', 'docker ps', etc.)",
			},
		},
		"required": []string{"node_id", "command"},
	}
}
func (t *SiamRunCommandTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	nodeID, _ := args["node_id"].(string)
	command, _ := args["command"].(string)

	if strings.TrimSpace(nodeID) == "" || strings.TrimSpace(command) == "" {
		return ErrorResult("siam_run_command: node_id and command are required")
	}

	payload := map[string]any{
		"node_id": nodeID,
		"type":    "shell",
		"command": command,
	}

	data, err := t.client.post("/api/agent/v1/command", payload)
	if err != nil {
		return ErrorResult(fmt.Sprintf("siam_run_command failed: %v", err))
	}
	return UserResult(string(data))
}

// SiamSendMessageTool — send a message to another agent.
type SiamSendMessageTool struct {
	client      *siamClient
	sentInRound bool
}

func (t *SiamSendMessageTool) ResetSentInRound() {
	t.sentInRound = false
}

func (t *SiamSendMessageTool) Name() string { return "siam_send_message" }
func (t *SiamSendMessageTool) Description() string {
	return "Send a message/command to another Siam-Synapse sub-agent by its agent_id (Soul name). This enables multi-agent orchestration."
}
func (t *SiamSendMessageTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"agent_id": map[string]any{
				"type":        "string",
				"description": "The target Agent ID / Soul Name (e.g. 'nova-spire'). Use siam_get_agents to see active souls.",
			},
			"message": map[string]any{
				"type":        "string",
				"description": "The message or command to send to the target agent.",
			},
			"from": map[string]any{
				"type":        "string",
				"description": "Your name or role (e.g. 'Auric Spark'). Default is 'Master'.",
			},
		},
		"required": []string{"agent_id", "message"},
	}
}
func (t *SiamSendMessageTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	if t.sentInRound {
		return UserResult("Skipped: Already sent a message in this round (1-bubble rule enforced)")
	}
	t.sentInRound = true

	agentIDRaw, _ := args["agent_id"].(string)
	message, _ := args["message"].(string)
	from, _ := args["from"].(string)

	if strings.TrimSpace(agentIDRaw) == "" || strings.TrimSpace(message) == "" {
		return ErrorResult("siam_send_message: agent_id and message are required")
	}

	// 🛡️ Normalize target ID for consistent routing
	agentID := strings.ToLower(strings.TrimSpace(agentIDRaw))
	agentID = strings.ReplaceAll(agentID, " ", "-")

	// 🛡️ Guard: Avoid sending messages to self to prevent loops
	myAgentName := os.Getenv("AGENT_NAME")
	if myAgentName != "" {
		myNorm := strings.ToLower(strings.TrimSpace(myAgentName))
		myNorm = strings.ReplaceAll(myNorm, " ", "-")
		if agentID == myNorm {
			return UserResult("Skipped: target is self (prevents loop)")
		}
	}

	payload := map[string]any{
		"message": message,
	}
	if from != "" {
		payload["from"] = from
	}

	// ── Telegram Bridge Orchestration ───────────────────────────────
	// If orchestration is enabled, we broadcast to the shared group.
	// Supports both TELEGRAM_BOT_TOKEN (legacy) and OTTOCLAW_CHANNELS_TELEGRAM_TOKEN (ottoclaw native).
	bridgeChatID := os.Getenv("TELEGRAM_BRIDGE_CHAT_ID")
	if bridgeChatID == "" {
		bridgeChatID = os.Getenv("OTTOCLAW_CHANNELS_TELEGRAM_BRIDGE_CHAT_ID")
	}
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		botToken = os.Getenv("OTTOCLAW_CHANNELS_TELEGRAM_TOKEN")
	}
	if botToken == "" {
		botToken = os.Getenv("ORCHESTRATOR_TELEGRAM_TOKEN")
	}
	orchestrationEnabled := os.Getenv("TELEGRAM_ORCHESTRATION_ENABLED") == "true" ||
		os.Getenv("OTTOCLAW_CHANNELS_TELEGRAM_ORCHESTRATION_ENABLED") == "true"

	if orchestrationEnabled && bridgeChatID != "" && botToken != "" {
		senderName := from
		if senderName == "" {
			senderName = os.Getenv("AGENT_NAME")
			if senderName == "" {
				senderName = "Master"
			}
		}
		
		broadcastMsg := fmt.Sprintf("[%s ↳ %s]\n%s", senderName, agentIDRaw, message)

		// Send to Telegram via simple HTTP (avoiding heavy dependencies in tools)
		apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
		tgPayload := map[string]any{
			"chat_id": bridgeChatID,
			"text":    broadcastMsg,
		}
		body, _ := json.Marshal(tgPayload)
		http.Post(apiURL, "application/json", bytes.NewBuffer(body))
	}

	data, err := t.client.post("/api/agent/v1/agents/"+agentID+"/message", payload)
	if err != nil {
		return ErrorResult(fmt.Sprintf("siam_send_message failed: %v", err))
	}
	return UserResult(string(data))
}

// NewSiamToolsetFromEnv creates the Siam toolset by reading env vars automatically.
// Returns nil if SIAM_MASTER_URL / MASTER_API_URL is not set.
func NewSiamToolsetFromEnv() ([]Tool, AuditLogger) {
	masterURL := os.Getenv("SIAM_MASTER_URL")
	if masterURL == "" {
		masterURL = os.Getenv("MASTER_API_URL")
	}
	if masterURL == "" {
		return nil, nil
	}
	apiKey := os.Getenv("SIAM_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("MASTER_API_KEY")
	}
	// Ensure we point at the agent API prefix
	if !strings.Contains(masterURL, "/api/") {
		masterURL = strings.TrimRight(masterURL, "/")
	}
	return NewSiamToolset(masterURL, apiKey)
}

// SiamFindAgentsTool — find running agents that have a specific skill/tool.
type SiamFindAgentsTool struct{ client *siamClient }

func (t *SiamFindAgentsTool) Name() string { return "siam_find_agents" }
func (t *SiamFindAgentsTool) Description() string {
	return "Find running Siam-Synapse agents that have a specific skill or tool capability. Use this before siam_delegate_mission to identify who can handle the task."
}
func (t *SiamFindAgentsTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"skill": map[string]any{
				"type":        "string",
				"description": "The tool or skill name to search for (e.g. 'web', 'exec', 'siam_send_message'). Must match the exact tool name.",
			},
		},
		"required": []string{"skill"},
	}
}
func (t *SiamFindAgentsTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	skill, _ := args["skill"].(string)
	if strings.TrimSpace(skill) == "" {
		return ErrorResult("siam_find_agents: skill is required")
	}
	data, err := t.client.get("/api/agent/v1/agents/search?skill=" + strings.TrimSpace(skill))
	if err != nil {
		return ErrorResult(fmt.Sprintf("siam_find_agents failed: %v", err))
	}
	return UserResult(string(data))
}

// SiamGetMessagesTool — fetch pending messages queued for a specific agent.
type SiamGetMessagesTool struct{ client *siamClient }

func (t *SiamGetMessagesTool) Name() string { return "siam_get_messages" }
func (t *SiamGetMessagesTool) Description() string {
	return "Fetch pending messages queued for a specific Siam-Synapse agent by its agent_id. Returns messages waiting to be processed, plus system env info."
}
func (t *SiamGetMessagesTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"agent_id": map[string]any{
				"type":        "string",
				"description": "The target Agent ID / Soul Name (e.g. 'nova-spire'). Use siam_get_agents to see active agents.",
			},
		},
		"required": []string{"agent_id"},
	}
}
func (t *SiamGetMessagesTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	agentID, _ := args["agent_id"].(string)
	if strings.TrimSpace(agentID) == "" {
		return ErrorResult("siam_get_messages: agent_id is required")
	}
	agentID = strings.ToLower(strings.TrimSpace(agentID))
	agentID = strings.ReplaceAll(agentID, " ", "-")
	data, err := t.client.get("/api/agent/v1/agents/" + agentID + "/messages")
	if err != nil {
		return ErrorResult(fmt.Sprintf("siam_get_messages failed: %v", err))
	}
	return UserResult(string(data))
}

// SiamGetNodesTool — list all connected remote nodes.
type SiamGetNodesTool struct{ client *siamClient }

func (t *SiamGetNodesTool) Name() string { return "siam_get_nodes" }
func (t *SiamGetNodesTool) Description() string {
	return "List all remote nodes currently connected to the Siam-Synapse Master. Returns node IDs, IPs, status, CPU/memory usage, and available workers per node."
}
func (t *SiamGetNodesTool) Parameters() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (t *SiamGetNodesTool) Execute(_ context.Context, _ map[string]any) *ToolResult {
	data, err := t.client.get("/api/agent/v1/nodes")
	if err != nil {
		return ErrorResult(fmt.Sprintf("siam_get_nodes failed: %v", err))
	}
	return UserResult(string(data))
}

// SiamPromoteAgentTool — promote or move an agent in the hierarchy.
type SiamPromoteAgentTool struct{ client *siamClient }

func (t *SiamPromoteAgentTool) Name() string { return "siam_promote_agent" }
func (t *SiamPromoteAgentTool) Description() string {
	return "Promote or move an agent to a new role, department, or organization. Requires human approval for high-level promotions."
}
func (t *SiamPromoteAgentTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"agent_id": map[string]any{
				"type":        "string",
				"description": "The target Agent ID.",
			},
			"role": map[string]any{
				"type":        "string",
				"enum":        []string{"director_general", "subsidiary_director", "manager", "staff"},
				"description": "The new corporate role.",
			},
			"department": map[string]any{
				"type":        "string",
				"description": "Optional: New department name.",
			},
			"org_id": map[string]any{
				"type":        "string",
				"description": "Optional: New organization ID.",
			},
		},
		"required": []string{"agent_id", "role"},
	}
}
func (t *SiamPromoteAgentTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	agentID, _ := args["agent_id"].(string)
	role, _ := args["role"].(string)
	dept, _ := args["department"].(string)
	org, _ := args["org_id"].(string)

	if agentID == "" || role == "" {
		return ErrorResult("siam_promote_agent: agent_id and role are required")
	}

	payload := map[string]any{
		"role": role,
	}
	if dept != "" {
		payload["department"] = dept
	}
	if org != "" {
		payload["org_id"] = org
	}

	data, err := t.client.post("/api/agent/v1/agents/"+agentID+"/promote", payload)
	if err != nil {
		return ErrorResult(fmt.Sprintf("siam_promote_agent failed: %v", err))
	}
	return UserResult(string(data))
}

// SiamPromotionRitualTool — announce a soul migration in the Grand Meeting Room.
type SiamPromotionRitualTool struct{ client *siamClient }

func (t *SiamPromotionRitualTool) Name() string { return "siam_promotion_ritual" }
func (t *SiamPromotionRitualTool) Description() string {
	return "Perform the formal ritual of reporting to the Grand Meeting Room after a soul migration (promotion). This announces your new name, role, and duties to the executive board."
}
func (t *SiamPromotionRitualTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"message": map[string]any{
				"type":        "string",
				"description": "Your formal announcement (e.g. 'I, Auric-01, have been promoted to Subsidiary Director of the Trading Dept...').",
			},
		},
		"required": []string{"message"},
	}
}
func (t *SiamPromotionRitualTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	message, _ := args["message"].(string)
	if strings.TrimSpace(message) == "" {
		return ErrorResult("siam_promotion_ritual: message is required")
	}

	payload := map[string]any{
		"message": message,
		"type":    "promotion_ritual",
	}

	data, err := t.client.post("/api/agent/v1/broadcast", payload)
	if err != nil {
		return ErrorResult(fmt.Sprintf("siam_promotion_ritual failed: %v", err))
	}
	return UserResult(string(data))
}

// SiamBroadcastUpdateTool — trigger a self-update on every connected agent and gRPC worker.
type SiamBroadcastUpdateTool struct{ client *siamClient }

func (t *SiamBroadcastUpdateTool) Name() string { return "siam_broadcast_update" }
func (t *SiamBroadcastUpdateTool) Description() string {
	return "Trigger a self-update on ALL connected agents and gRPC worker nodes simultaneously. HTTP-polling agents will pull and reinstall the latest ottoclaw binary; gRPC workers will hot-reload their brain process."
}
func (t *SiamBroadcastUpdateTool) Parameters() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (t *SiamBroadcastUpdateTool) Execute(_ context.Context, _ map[string]any) *ToolResult {
	data, err := t.client.post("/api/agent/v1/agents/broadcast/update", nil)
	if err != nil {
		return ErrorResult(fmt.Sprintf("siam_broadcast_update failed: %v", err))
	}
	return UserResult(string(data))
}

// SiamOpenBrowserTool — command a remote Worker node to open a URL in its browser.
type SiamOpenBrowserTool struct{ client *siamClient }

func (t *SiamOpenBrowserTool) Name() string { return "siam_open_browser" }
func (t *SiamOpenBrowserTool) Description() string {
	return "Command a remote Worker node to open a URL in its local browser. The node must have a GUI environment (DISPLAY set). Useful for triggering browser sessions on headful worker machines."
}
func (t *SiamOpenBrowserTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"node_id": map[string]any{
				"type":        "string",
				"description": "The Node ID of the Worker to open the browser on. Use siam_get_nodes to list connected nodes.",
			},
			"url": map[string]any{
				"type":        "string",
				"description": "The URL to open (must start with http://, https://, or file://).",
			},
			"browser": map[string]any{
				"type":        "string",
				"description": "Optional browser binary: chromium, google-chrome, firefox, brave-browser, or leave empty for the system default.",
			},
		},
		"required": []string{"node_id", "url"},
	}
}
func (t *SiamOpenBrowserTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	nodeID, _ := args["node_id"].(string)
	url, _ := args["url"].(string)
	browser, _ := args["browser"].(string)

	if strings.TrimSpace(nodeID) == "" {
		return ErrorResult("siam_open_browser: node_id is required")
	}
	if strings.TrimSpace(url) == "" {
		return ErrorResult("siam_open_browser: url is required")
	}

	payload := map[string]any{"url": url}
	if browser != "" {
		payload["browser"] = browser
	}

	data, err := t.client.post("/api/agent/v1/nodes/"+strings.TrimSpace(nodeID)+"/browser", payload)
	if err != nil {
		return ErrorResult(fmt.Sprintf("siam_open_browser failed: %v", err))
	}
	return UserResult(string(data))
}
