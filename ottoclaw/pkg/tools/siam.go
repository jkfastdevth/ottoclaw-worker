package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// siamClient is a shared HTTP helper for Siam-Synapse Master API calls.
type siamClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func newSiamClient(baseURL, apiKey string) *siamClient {
	return &siamClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 15 * time.Second},
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

// NewSiamToolset creates all Siam-Synapse tools, reading SIAM_MASTER_URL
// and SIAM_API_KEY (or MASTER_API_URL / MASTER_API_KEY) from the environment.
func NewSiamToolset(masterURL, apiKey string) []Tool {
	client := newSiamClient(masterURL, apiKey)
	return []Tool{
		&SiamGetMetricsTool{client: client},
		&SiamGetAgentsTool{client: client},
		&SiamSpawnAgentTool{client: client},
		&SiamTerminateAgentTool{client: client},
		&SiamGetSkillsTool{client: client},
		&SiamScaleTool{client: client},
		&SiamGetJobsTool{client: client},
		&SiamSubmitJobTool{client: client},
		&SiamRunCommandTool{client: client},
		&SiamSendMessageTool{client: client},
	}
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
type SiamSendMessageTool struct{ client *siamClient }

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
	agentIDRaw, _ := args["agent_id"].(string)
	message, _ := args["message"].(string)
	from, _ := args["from"].(string)

	if strings.TrimSpace(agentIDRaw) == "" || strings.TrimSpace(message) == "" {
		return ErrorResult("siam_send_message: agent_id and message are required")
	}

	// 🛡️ Normalize target ID for consistent routing
	agentID := strings.ToLower(strings.TrimSpace(agentIDRaw))
	agentID = strings.ReplaceAll(agentID, " ", "-")

	payload := map[string]any{
		"message": message,
	}
	if from != "" {
		payload["from"] = from
	}

	// ── Telegram Bridge Orchestration ───────────────────────────────
	// If orchestration is enabled, we broadcast to the shared group
	bridgeChatID := os.Getenv("TELEGRAM_BRIDGE_CHAT_ID")
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if os.Getenv("TELEGRAM_ORCHESTRATION_ENABLED") == "true" && bridgeChatID != "" && botToken != "" {
		broadcastMsg := fmt.Sprintf("%s %s", agentID, message)

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
// Returns nil slice if SIAM_MASTER_URL / MASTER_API_URL is not set.
func NewSiamToolsetFromEnv() []Tool {
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
	// Ensure we point at the agent API prefix
	if !strings.Contains(masterURL, "/api/") {
		masterURL = strings.TrimRight(masterURL, "/")
	}
	return NewSiamToolset(masterURL, apiKey)
}
