package siam

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

	"github.com/sipeed/ottoclaw/pkg/bus"
	"github.com/sipeed/ottoclaw/pkg/channels"
	"github.com/sipeed/ottoclaw/pkg/config"
	"github.com/sipeed/ottoclaw/pkg/logger"
)

func init() {
	channels.RegisterFactory("siam_sync", func(cfg *config.Config, b *bus.MessageBus) (channels.Channel, error) {
		return NewSiamSyncChannel(cfg, b), nil
	})
}

type SiamSyncChannel struct {
	*channels.BaseChannel
	cfg      *config.Config
	pollDone chan struct{}
}

func NewSiamSyncChannel(cfg *config.Config, b *bus.MessageBus) *SiamSyncChannel {
	// Identity check: use AGENT_NAME as the primary soul identity
	agentName := os.Getenv("AGENT_NAME")
	if agentName == "" {
		agentName = "unknown"
	}

	return &SiamSyncChannel{
		BaseChannel: channels.NewBaseChannel("siam_sync", cfg.Channels.SiamSync, b, cfg.Channels.SiamSync.AllowFrom),
		cfg:         cfg,
		pollDone:    make(chan struct{}),
	}
}

func (s *SiamSyncChannel) Start(ctx context.Context) error {
	s.SetRunning(true)
	go s.pollLoop(ctx)
	return nil
}

func (s *SiamSyncChannel) Stop(ctx context.Context) error {
	s.SetRunning(false)
	close(s.pollDone)
	return nil
}

func (s *SiamSyncChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	// ── Telegram Bridge Broadcast ───────────────────────────────────
	// Since SiamSync is inbound-only for the Master API queue, 
	// we use direct Telegram broadcast for the OUTBOUND response.
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

	if orchestrationEnabled && bridgeChatID != "" && botToken != "" && msg.Content != "" {
		agentName := os.Getenv("AGENT_NAME")
		if agentName == "" {
			agentName = "Worker"
		}
		
		target := msg.ChatID
		if target == "" || target == "direct" {
			target = "User"
		}

		broadcastMsg := fmt.Sprintf("[%s ↳ %s]\n%s", agentName, target, msg.Content)

		apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
		tgPayload := map[string]any{
			"chat_id": bridgeChatID,
			"text":    broadcastMsg,
		}
		body, _ := json.Marshal(tgPayload)
		
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Post(apiURL, "application/json", bytes.NewBuffer(body))
		if err != nil {
			logger.WarnCF("siam_sync", "Failed to broadcast response to Telegram", map[string]any{"error": err.Error()})
			return nil // Don't fail the whole loop for broadcast failure
		}
		if resp != nil {
			resp.Body.Close()
		}
		logger.InfoCF("siam_sync", "Broadcasted response to Telegram", map[string]any{"target": target})
	}

	return nil
}

func (s *SiamSyncChannel) pollLoop(ctx context.Context) {
	defer logger.InfoC("siam_sync", "Polling loop stopped")

	interval := time.Duration(s.cfg.Channels.SiamSync.Interval) * time.Second
	if interval < 1*time.Second {
		interval = 5 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	masterURL := s.cfg.Channels.SiamSync.MasterURL
	if masterURL == "" {
		masterURL = os.Getenv("MASTER_API_URL")
	}
	if masterURL == "" {
		masterURL = "http://master:8080"
	}

	apiKey := s.cfg.Channels.SiamSync.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("MASTER_API_KEY")
	}

	agentName := os.Getenv("AGENT_NAME")
	if agentName == "" {
		logger.ErrorC("siam_sync", "AGENT_NAME not set, polling disabled")
		return
	}

	logger.InfoCF("siam_sync", "Starting polling loop", map[string]any{
		"agent":      agentName,
		"master_url": masterURL,
		"interval":   interval.String(),
	})

	// 🚀 Clock-in: Send onboarding message to Telegram Bridge
	s.sendOnboardingMessage(agentName)


	for {
		select {
		case <-ctx.Done():
			return
		case <-s.pollDone:
			return
		case <-ticker.C:
			s.fetchMessages(ctx, masterURL, apiKey, agentName)
		}
	}
}

type siamMessage struct {
	ID        string            `json:"id"`
	Sender    string            `json:"sender"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata"`
	CreatedAt time.Time         `json:"created_at"`
}

func (s *SiamSyncChannel) fetchMessages(ctx context.Context, masterURL, apiKey, agentName string) {
	// Normalize agent name to match master's routing key format (e.g. "Auric Spark" → "auric-spark")
	agentKey := strings.ToLower(strings.TrimSpace(agentName))
	agentKey = strings.NewReplacer(" ", "-", "_", "-").Replace(agentKey)
	url := fmt.Sprintf("%s/api/agent/v1/agents/%s/messages", masterURL, agentKey)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		logger.ErrorCF("siam_sync", "Failed to create request", map[string]any{"error": err.Error()})
		return
	}

	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		if !strings.Contains(err.Error(), "connection refused") {
			logger.WarnCF("siam_sync", "Failed to poll Master", map[string]any{"error": err.Error(), "url": url})
		}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNoContent {
			return
		}
		body, _ := io.ReadAll(resp.Body)
		logger.WarnCF("siam_sync", "Master returned error", map[string]any{
			"status": resp.StatusCode,
			"body":   string(body),
		})
		return
	}

	var result struct {
		Messages  []string          `json:"messages"`
		SystemEnv map[string]string `json:"system_env"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logger.ErrorCF("siam_sync", "Failed to decode messages", map[string]any{"error": err.Error()})
		return
	}

	// ── Inject System Environment Variables ────────────────────────
	if len(result.SystemEnv) > 0 {
		for k, v := range result.SystemEnv {
			if v != "" {
				os.Setenv(k, v)
				//logger.DebugCF("siam_sync", "Injected system env", map[string]any{"key": k})
			}
		}
		logger.InfoCF("siam_sync", "Injected system environment variables", map[string]any{"count": len(result.SystemEnv)})
	}

	for _, m := range result.Messages {
		logger.InfoCF("siam_sync", "Received message from Siam", map[string]any{
			"content": m,
		})

		// Parse formatted string "[Sender]: Content"
		sender := "Master"
		content := m
		if strings.HasPrefix(m, "[") {
			if idx := strings.Index(m, "]: "); idx > 0 {
				sender = m[1:idx]
				content = m[idx+3:]
			}
		}

		if content == "SYSTEM_HOT_RELOAD" {
			logger.WarnCF("siam_sync", "🚨 SYSTEM_HOT_RELOAD triggered, restarting Agent binary to load full config setup...", nil)
			// Trigger Exit with code 1 so systemd or supervisor restarts it automatically (Restart=on-failure)
			os.Exit(1)
		}

		// Inject into bus
		peer := bus.Peer{
			ID:   sender,
			Kind: "user",
		}

		s.HandleMessage(
			ctx,
			peer,
			fmt.Sprintf("siam-%d", time.Now().UnixNano()),
			sender,
			"direct",
			content,
			nil,
			nil,
		)
	}
}

func (s *SiamSyncChannel) sendOnboardingMessage(agentName string) {
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
		onboardingMsg := fmt.Sprintf("[%s]: รายงานตัวเข้างานครับ! พร้อมรับคำสั่งแล้ว 💼", agentName)

		apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
		tgPayload := map[string]any{
			"chat_id": bridgeChatID,
			"text":    onboardingMsg,
		}
		body, _ := json.Marshal(tgPayload)
		
		go func() {
			resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(body))
			if err == nil && resp != nil {
				resp.Body.Close()
				logger.InfoCF("siam_sync", "Clock-in message sent", map[string]any{"agent": agentName})
			}
		}()
	}
}
