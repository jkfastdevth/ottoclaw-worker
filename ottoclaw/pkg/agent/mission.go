package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"os/exec"
	"os/signal"
	"syscall"

	"github.com/sipeed/ottoclaw/pkg/bus"
	"github.com/sipeed/ottoclaw/pkg/config"
	"github.com/sipeed/ottoclaw/pkg/logger"
)

type Mission struct {
	ID           string    `json:"id"`
	AgentID      string    `json:"agent_id"`
	Description  string    `json:"description"`
	ParentID     string    `json:"parent_id"`
	Status       string    `json:"status"`
	Result       string    `json:"result"`
	Checkpoint   string    `json:"checkpoint"`
	NotifyTarget string    `json:"notify_target"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
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
		// Try to load from SOUL_ID file in workspace
		workspaceDir := os.Getenv("OTTOCLAW_WORKSPACE")
		if workspaceDir == "" {
			workspaceDir = "/app/workspace"
		}
		soulIDPath := filepath.Join(workspaceDir, "SOUL_ID")
		if data, err := os.ReadFile(soulIDPath); err == nil && len(data) > 0 {
			agentID = strings.TrimSpace(string(data))
			agentID = strings.ToLower(agentID)
		}
	}
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

func isOlder(current, latest string) bool {
	if current == "" || latest == "" {
		return false
	}
	partsCurrent := strings.Split(strings.TrimPrefix(current, "v"), ".")
	partsLatest := strings.Split(strings.TrimPrefix(latest, "v"), ".")

	for i := 0; i < len(partsCurrent) && i < len(partsLatest); i++ {
		var c, l int
		pC := partsCurrent[i]
		pL := partsLatest[i]

		if idx := strings.Index(pC, "-"); idx != -1 {
			pC = pC[:idx]
		}
		if idx := strings.Index(pL, "-"); idx != -1 {
			pL = pL[:idx]
		}

		for _, r := range pC {
			if r >= '0' && r <= '9' {
				c = c*10 + int(r-'0')
			}
		}
		for _, r := range pL {
			if r >= '0' && r <= '9' {
				l = l*10 + int(r-'0')
			}
		}

		if c < l {
			return true
		}
		if c > l {
			return false
		}
	}
	return len(partsCurrent) < len(partsLatest)
}

func (m *MissionManager) getVersion() string {
	// 1. Try to get git commit hash first (more accurate for source updates)
	repoDir := m.cfg.WorkspacePath()
	if repoDir == "" {
		repoDir = "."
	}
	cmd := exec.Command("git", "-C", repoDir, "rev-parse", "--short", "HEAD")
	if out, err := cmd.Output(); err == nil {
		return strings.TrimSpace(string(out))
	}

	// 2. Fallback to etc version file
	data, err := os.ReadFile("/etc/ottoclaw/version")
	if err != nil {
		return "latest"
	}
	return strings.TrimSpace(string(data))
}

func (m *MissionManager) collectSystemSpec() map[string]any {
	spec := make(map[string]any)
	spec["os"] = runtime.GOOS
	spec["arch"] = runtime.GOARCH

	// 🕵️ Detect Camera
	if runtime.GOOS == "linux" {
		// Try termux-camera-info
		cmd := exec.Command("termux-camera-info")
		if err := cmd.Run(); err == nil {
			spec["camera"] = "available (termux)"
		} else {
			// Check /dev/video*
			files, _ := filepath.Glob("/dev/video*")
			if len(files) > 0 {
				spec["camera"] = "available (v4l2)"
			}
		}

		// 🔋 Battery
		if _, err := os.Stat("/sys/class/power_supply/battery/capacity"); err == nil {
			spec["battery"] = "available (sysfs)"
		} else if cmd := exec.Command("termux-battery-status"); cmd.Run() == nil {
			spec["battery"] = "available (termux)"
		}

		// 🌡️ Thermal
		if _, err := os.Stat("/sys/class/thermal/thermal_zone0/temp"); err == nil {
			spec["thermal"] = "available (sysfs)"
		}
	}

	return spec
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

		upStatus, upErr := GetUpdateStatus()

		// request node_secret only when it's missing from config (avoids broadcasting on every heartbeat)
		needNodeSecret := m.cfg.Channels.SiamSync.NodeSecret == "" && os.Getenv("NODE_SECRET") == ""

		spec := m.collectSystemSpec()

		payload := map[string]any{
			"today_usage":      usage,
			"today_cost":       cost,
			"max_daily_tokens": agent.MaxDailyTokens,
			"tools":            agent.Tools.List(),
			"version":          m.getVersion(),
			"update_status":    upStatus,
			"update_error":     upErr,
			"need_node_secret": needNodeSecret,
			"system_spec":      spec,
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
			var respData struct {
				Status        string `json:"status"`
				LatestVersion string `json:"latest_version,omitempty"`
				NodeSecret    string `json:"node_secret,omitempty"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&respData); err == nil {
				if respData.Status == "limbo" {
					logger.WarnCF("mission", "🚨 Agent is in LIMBO! Requesting master respawn...", map[string]any{"agent_id": agent.ID})
					go m.requestRespawn(ctx, masterURL, apiKey, agent.ID)
				}

				// 🔐 Apply node_secret from master if not already set in config
				if respData.NodeSecret != "" && m.cfg.Channels.SiamSync.NodeSecret != respData.NodeSecret {
					m.cfg.Channels.SiamSync.NodeSecret = respData.NodeSecret
					if home, herr := os.UserHomeDir(); herr == nil {
						cfgPath := filepath.Join(home, ".ottoclaw", "config.json")
						if serr := config.SaveConfig(cfgPath, m.cfg); serr != nil {
							logger.WarnCF("mission", "Failed to persist node_secret to config", map[string]any{"error": serr.Error()})
						} else {
							logger.InfoC("mission", "✅ Config patched: node_secret updated from master")
						}
					}
				}

				// 🚀 Auto update checking
				currentVer := m.getVersion()
				if respData.LatestVersion != "" && isOlder(currentVer, respData.LatestVersion) {
					logger.InfoCF("mission", "🚀 Newer version found, triggering auto-update", map[string]any{
						"current": currentVer,
						"latest":  respData.LatestVersion,
					})
					inbound := bus.InboundMessage{
						Channel: "mission",
						Content: "ottoclaw update",
						Peer:    bus.Peer{ID: "Master", Kind: "user"},
					}
					m.bus.PublishInbound(ctx, inbound)
				} else {
					// Clear status only if no trigger is raised and heartbeat was acknowledged
					ClearUpdateStatus()
				}
			}
			resp.Body.Close()
		}
	}
}

func (m *MissionManager) requestRespawn(ctx context.Context, masterURL, apiKey, agentID string) {
	url := fmt.Sprintf("%s/api/agent/v1/agents/%s/respawn", masterURL, agentID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return
	}

	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.ErrorCF("mission", "Failed to Request Respawn", map[string]any{"agent_id": agentID, "error": err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		logger.InfoCF("mission", "Successfully requested Respawn from Master", map[string]any{"agent_id": agentID})
	} else {
		logger.WarnCF("mission", "Respawn request returned non-200", map[string]any{"agent_id": agentID, "status": resp.StatusCode})
	}
}

func (m *MissionManager) pollMissions(ctx context.Context, masterURL, apiKey string) {
	url := fmt.Sprintf("%s/api/agent/v1/missions/%s?status=pending", masterURL, m.agentID)
	logger.InfoCF("mission", "Polling URL", map[string]any{"url": url})

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
		logger.WarnCF("mission", "Poll failed or returned non-200", map[string]any{"status": resp.StatusCode})
		return
	}

	var missions []Mission
	if err := json.NewDecoder(resp.Body).Decode(&missions); err != nil {
		logger.WarnCF("mission", "Failed to decode missions", map[string]any{"error": err.Error()})
		return
	}

	logger.InfoCF("mission", "Poll complete", map[string]any{"count": len(missions)})

	for _, mission := range missions {
		m.injectMission(ctx, mission)
	}
}

// markInProgress PATCHes the mission status to "in_progress" on the Master
// so it no longer appears in the pending poll, preventing infinite re-polling.
func (m *MissionManager) markInProgress(ctx context.Context, missionID string) error {
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
	payload := map[string]string{"status": "in_progress"}
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
		return fmt.Errorf("master returned status %d when marking in_progress", resp.StatusCode)
	}
	return nil
}

func (m *MissionManager) injectMission(ctx context.Context, mission Mission) {
	logger.InfoCF("mission", "Received mission from Master", map[string]any{
		"id":          mission.ID,
		"description": mission.Description,
	})

	// ⭐ Mark as in_progress FIRST so Master stops returning this in pending polls.
	// This is the critical fix for the infinite re-poll loop.
	if err := m.markInProgress(ctx, mission.ID); err != nil {
		logger.WarnCF("mission", "Failed to mark mission in_progress — will still attempt execution", map[string]any{
			"mission_id": mission.ID,
			"error":      err.Error(),
		})
	} else {
		logger.InfoCF("mission", "Mission marked in_progress", map[string]any{"mission_id": mission.ID})
	}

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
			"mission_id":    mission.ID,
			"parent_id":     mission.ParentID,
			"checkpoint":    mission.Checkpoint,
			"type":          "mission",
			"notify_target": mission.NotifyTarget,
		},
	}

	if err := m.bus.PublishInbound(ctx, inbound); err != nil {
		logger.ErrorCF("mission", "Failed to publish mission to bus", map[string]any{"error": err.Error()})
	}
}

func (m *MissionManager) ReportResult(ctx context.Context, missionID string, success bool, output string, notifyTarget ...string) error {
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
	if !success {
		payload["error"] = output
	}
	if len(notifyTarget) > 0 && notifyTarget[0] != "" {
		payload["notify_target"] = notifyTarget[0]
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
