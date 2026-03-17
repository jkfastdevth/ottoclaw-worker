package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"os/signal"
	"syscall"

	"github.com/sipeed/ottoclaw/pkg/bus"
	"github.com/sipeed/ottoclaw/pkg/config"
	"github.com/sipeed/ottoclaw/pkg/logger"
)

type Mission struct {
	ID          string    `json:"id"`
	AgentID     string    `json:"agent_id"`
	Description string    `json:"description"`
	ParentID    string    `json:"parent_id"`
	Status      string    `json:"status"`
	Result      string    `json:"result"`
	Checkpoint  string    `json:"checkpoint"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type MissionManager struct {
	cfg      *config.Config
	bus      *bus.MessageBus
	registry *AgentRegistry
	agentID  string
	interval time.Duration
}

func NewMissionManager(cfg *config.Config, b *bus.MessageBus, r *AgentRegistry) *MissionManager {
	agentID := os.Getenv("AGENT_NAME")
	if agentID == "" {
		agentID = "unknown"
	}

	interval := 30 * time.Second
	if cfg.Channels.SiamSync.Interval > 0 {
		interval = time.Duration(cfg.Channels.SiamSync.Interval) * time.Second
	}

	return &MissionManager{
		cfg:      cfg,
		bus:      b,
		registry: r,
		agentID:  agentID,
		interval: interval,
	}
}

func (m *MissionManager) Start(ctx context.Context) {
	if m.agentID == "unknown" {
		logger.WarnC("mission", "AGENT_NAME not set, mission polling disabled")
		return
	}

	masterURL := m.cfg.Channels.SiamSync.MasterURL
	if masterURL == "" {
		masterURL = os.Getenv("MASTER_API_URL")
	}
	if masterURL == "" {
		masterURL = "http://master:8080"
	}

	apiKey := m.cfg.Channels.SiamSync.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("MASTER_API_KEY")
	}

	logger.InfoCF("mission", "Starting mission polling loop", map[string]any{
		"agent":    m.agentID,
		"interval": m.interval.String(),
	})

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	// 💓 Heartbeat ticker (every 60s)
	heartbeatTicker := time.NewTicker(60 * time.Second)
	defer heartbeatTicker.Stop()

	// Initial heartbeat
	m.reportHeartbeats(ctx, masterURL, apiKey)

	// 🚀 Phase 6: Listen for SIGUSR1 to trigger immediate poll (Master Nudge)
	nudgeChan := make(chan os.Signal, 1)
	signal.Notify(nudgeChan, syscall.SIGUSR1)
	defer signal.Stop(nudgeChan)

	for {
		select {
		case <-ctx.Done():
			return
		case <-nudgeChan:
			logger.InfoC("mission", "🚀 Received SIGUSR1 nudge! Polling missions immediately...")
			m.pollMissions(ctx, masterURL, apiKey)
		case <-ticker.C:
			m.pollMissions(ctx, masterURL, apiKey)
		case <-heartbeatTicker.C:
			m.reportHeartbeats(ctx, masterURL, apiKey)
		}
	}
}

func (m *MissionManager) reportHeartbeats(ctx context.Context, masterURL, apiKey string) {
	if m.registry == nil {
		return
	}

	m.registry.mu.RLock()
	agents := make([]*AgentInstance, 0, len(m.registry.agents))
	for _, a := range m.registry.agents {
		agents = append(agents, a)
	}
	m.registry.mu.RUnlock()

	client := &http.Client{Timeout: 5 * time.Second}

	for _, agent := range agents {
		// Get stats
		usage := 0
		cost := 0.0
		if agent.Ledger != nil {
			usage = agent.Ledger.GetTodayUsage()
			cost = agent.Ledger.GetEstimatedCost()
		}

		payload := map[string]any{
			"today_usage":      usage,
			"today_cost":       cost,
			"max_daily_tokens": agent.MaxDailyTokens,
			"tools":            agent.Tools.List(),
		}
		body, _ := json.Marshal(payload)

		url := fmt.Sprintf("%s/api/agent/v1/agents/%s/heartbeat", masterURL, agent.ID)
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
		if err != nil {
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		if apiKey != "" {
			req.Header.Set("X-API-Key", apiKey)
		}

		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}
}

func (m *MissionManager) pollMissions(ctx context.Context, masterURL, apiKey string) {
	url := fmt.Sprintf("%s/api/agent/v1/missions/%s?status=pending", masterURL, m.agentID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return
	}

	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var missions []Mission
	if err := json.NewDecoder(resp.Body).Decode(&missions); err != nil {
		return
	}

	for _, mission := range missions {
		m.injectMission(ctx, mission)
	}
}

func (m *MissionManager) injectMission(ctx context.Context, mission Mission) {
	logger.InfoCF("mission", "Received mission from Master", map[string]any{
		"id":          mission.ID,
		"description": mission.Description,
	})

	// Inject into bus as an inbound message
	peer := bus.Peer{
		ID:   "Master",
		Kind: "user",
	}

	inbound := bus.InboundMessage{
		Channel:    "mission",
		SenderID:   "Master",
		ChatID:     "mission-control",
		Content:    mission.Description,
		Peer:       peer,
		MessageID:  mission.ID, // Use Mission ID as Message ID
		SessionKey: "mission-" + mission.ID,
		Metadata: map[string]string{
			"mission_id": mission.ID,
			"parent_id":  mission.ParentID,
			"checkpoint": mission.Checkpoint,
			"type":       "mission",
		},
	}

	if err := m.bus.PublishInbound(ctx, inbound); err != nil {
		logger.ErrorCF("mission", "Failed to publish mission to bus", map[string]any{"error": err.Error()})
	}
}

func (m *MissionManager) ReportResult(ctx context.Context, missionID string, success bool, output string) error {
	masterURL := m.cfg.Channels.SiamSync.MasterURL
	if masterURL == "" {
		masterURL = os.Getenv("MASTER_API_URL")
	}
	if masterURL == "" {
		masterURL = "http://master:8080"
	}

	apiKey := m.cfg.Channels.SiamSync.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("MASTER_API_KEY")
	}

	url := fmt.Sprintf("%s/api/agent/v1/missions/%s", masterURL, missionID)
	status := "completed"
	if !success {
		status = "failed"
	}

	payload := map[string]string{
		"status": status,
		"result": output,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("master returned status %d", resp.StatusCode)
	}

	return nil
}

func (m *MissionManager) ReportCheckpoint(ctx context.Context, missionID string, checkpoint string) error {
	masterURL := m.cfg.Channels.SiamSync.MasterURL
	if masterURL == "" {
		masterURL = os.Getenv("MASTER_API_URL")
	}
	if masterURL == "" {
		masterURL = "http://master:8080"
	}

	apiKey := m.cfg.Channels.SiamSync.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("MASTER_API_KEY")
	}

	url := fmt.Sprintf("%s/api/agent/v1/missions/%s", masterURL, missionID)
	payload := map[string]string{
		"status":     "running",
		"checkpoint": checkpoint,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("master returned status %d", resp.StatusCode)
	}

	return nil
}
func (m *MissionManager) SearchKnowledge(ctx context.Context, query string) (string, error) {
	masterURL := m.cfg.Channels.SiamSync.MasterURL
	if masterURL == "" {
		masterURL = os.Getenv("MASTER_API_URL")
	}
	if masterURL == "" {
		masterURL = "http://master:8080"
	}

	apiKey := m.cfg.Channels.SiamSync.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("MASTER_API_KEY")
	}

	url := fmt.Sprintf("%s/api/agent/v1/knowledge/search?query=%s&limit=3", masterURL, query)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("master returned status %d", resp.StatusCode)
	}

	var entries []struct {
		Fact       string  `json:"fact"`
		Confidence float64 `json:"confidence"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return "", err
	}

	if len(entries) == 0 {
		return "", nil
	}

	var sb strings.Builder
	sb.WriteString("\n[Akashic Library - Relevant Context]\n")
	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("- %s (Confidence: %.2f)\n", e.Fact, e.Confidence))
	}
	return sb.String(), nil
}
