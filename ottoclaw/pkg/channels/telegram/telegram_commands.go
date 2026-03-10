package telegram

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mymmrac/telego"

	"net/http"
	"bytes"
	"encoding/json"
	"io"

	"github.com/sipeed/ottoclaw/pkg/config"
)

type TelegramCommander interface {
	Help(ctx context.Context, message telego.Message) error
	Start(ctx context.Context, message telego.Message) error
	Show(ctx context.Context, message telego.Message) error
	List(ctx context.Context, message telego.Message) error
	Soul(ctx context.Context, message telego.Message) error
}

type cmd struct {
	bot    *telego.Bot
	config *config.Config
}

func NewTelegramCommands(bot *telego.Bot, cfg *config.Config) TelegramCommander {
	return &cmd{
		bot:    bot,
		config: cfg,
	}
}

func commandArgs(text string) string {
	parts := strings.SplitN(text, " ", 2)
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func (c *cmd) Help(ctx context.Context, message telego.Message) error {
	msg := `/start - Start the bot
/help - Show this help message
/soul - View current Soul identity
/soul [content] - Manually update the Soul identity
/show [model|channel] - Show current configuration
/list [models|channels] - List available options
	`
	_, err := c.bot.SendMessage(ctx, &telego.SendMessageParams{
		ChatID: telego.ChatID{ID: message.Chat.ID},
		Text:   msg,
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
	return err
}

func (c *cmd) Start(ctx context.Context, message telego.Message) error {
	_, err := c.bot.SendMessage(ctx, &telego.SendMessageParams{
		ChatID: telego.ChatID{ID: message.Chat.ID},
		Text:   "Hello! I am OttoClaw 🦞",
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
	return err
}

func (c *cmd) Show(ctx context.Context, message telego.Message) error {
	args := commandArgs(message.Text)
	if args == "" {
		_, err := c.bot.SendMessage(ctx, &telego.SendMessageParams{
			ChatID: telego.ChatID{ID: message.Chat.ID},
			Text:   "Usage: /show [model|channel]",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return err
	}

	var response string
	switch args {
	case "model":
		response = fmt.Sprintf("Current Model: %s (Provider: %s)",
			c.config.Agents.Defaults.GetModelName(),
			c.config.Agents.Defaults.Provider)
	case "channel":
		response = "Current Channel: telegram"
	default:
		response = fmt.Sprintf("Unknown parameter: %s. Try 'model' or 'channel'.", args)
	}

	_, err := c.bot.SendMessage(ctx, &telego.SendMessageParams{
		ChatID: telego.ChatID{ID: message.Chat.ID},
		Text:   response,
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
	return err
}

func (c *cmd) List(ctx context.Context, message telego.Message) error {
	args := commandArgs(message.Text)
	if args == "" {
		_, err := c.bot.SendMessage(ctx, &telego.SendMessageParams{
			ChatID: telego.ChatID{ID: message.Chat.ID},
			Text:   "Usage: /list [models|channels]",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return err
	}

	var response string
	switch args {
	case "models":
		provider := c.config.Agents.Defaults.Provider
		if provider == "" {
			provider = "configured default"
		}
		response = fmt.Sprintf("Configured Model: %s\nProvider: %s\n\nTo change models, update config.json",
			c.config.Agents.Defaults.GetModelName(), provider)

	case "channels":
		var enabled []string
		if c.config.Channels.Telegram.Enabled {
			enabled = append(enabled, "telegram")
		}
		if c.config.Channels.WhatsApp.Enabled {
			enabled = append(enabled, "whatsapp")
		}
		if c.config.Channels.Feishu.Enabled {
			enabled = append(enabled, "feishu")
		}
		if c.config.Channels.Discord.Enabled {
			enabled = append(enabled, "discord")
		}
		if c.config.Channels.Slack.Enabled {
			enabled = append(enabled, "slack")
		}
		response = fmt.Sprintf("Enabled Channels:\n- %s", strings.Join(enabled, "\n- "))

	default:
		response = fmt.Sprintf("Unknown parameter: %s. Try 'models' or 'channels'.", args)
	}

	_, err := c.bot.SendMessage(ctx, &telego.SendMessageParams{
		ChatID: telego.ChatID{ID: message.Chat.ID},
		Text:   response,
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
	return err
}

func (c *cmd) Soul(ctx context.Context, message telego.Message) error {
	workspace := os.Getenv("OTTOCLAW_WORKSPACE")
	if workspace == "" {
		workspace = "/app/workspace/v2"
	}
	soulPath := workspace + "/SOUL.md"

	args := commandArgs(message.Text)
	if args == "" {
		// View mode
		content, err := os.ReadFile(soulPath)
		if err != nil {
			_, err = c.bot.SendMessage(ctx, &telego.SendMessageParams{
				ChatID: telego.ChatID{ID: message.Chat.ID},
				Text:   "⚠️ Failed to read Soul records: " + err.Error(),
			})
			return err
		}
		_, err = c.bot.SendMessage(ctx, &telego.SendMessageParams{
			ChatID:    telego.ChatID{ID: message.Chat.ID},
			Text:      "📜 *Current Soul Identity:*\n\n" + string(content),
			ParseMode: telego.ModeMarkdown,
		})
		return err
	}

	if strings.HasPrefix(args, "reincarnate") {
		newName := strings.TrimSpace(strings.TrimPrefix(args, "reincarnate"))
		
		masterURL := c.config.Channels.SiamSync.MasterURL
		if masterURL == "" {
			masterURL = os.Getenv("MASTER_API_URL")
		}
		apiKey := c.config.Channels.SiamSync.APIKey
		if apiKey == "" {
			apiKey = os.Getenv("MASTER_API_KEY")
		}

		if masterURL == "" {
			_, err := c.bot.SendMessage(ctx, &telego.SendMessageParams{
				ChatID: telego.ChatID{ID: message.Chat.ID},
				Text:   "❌ Error: MASTER_API_URL is not configured on this worker.",
			})
			return err
		}

		// Identify self
		agentID := os.Getenv("AGENT_NAME")
		if agentID == "" && len(c.config.Agents.List) > 0 {
			agentID = c.config.Agents.List[0].ID
		}
		// Normalize ID
		agentID = strings.ToLower(strings.TrimSpace(agentID))
		agentID = strings.ReplaceAll(agentID, " ", "-")

		if agentID == "" {
			_, err := c.bot.SendMessage(ctx, &telego.SendMessageParams{
				ChatID: telego.ChatID{ID: message.Chat.ID},
				Text:   "❌ Error: Could not determine current Agent ID.",
			})
			return err
		}

		payload := map[string]string{
			"name": newName,
		}
		body, _ := json.Marshal(payload)
		
		apiEndpoint := fmt.Sprintf("%s/api/agent/v1/agents/%s/reincarnate", strings.TrimRight(masterURL, "/"), agentID)
		
		req, err := http.NewRequest("POST", apiEndpoint, bytes.NewBuffer(body))
		if err != nil {
			_, err = c.bot.SendMessage(ctx, &telego.SendMessageParams{
				ChatID: telego.ChatID{ID: message.Chat.ID},
				Text:   "❌ Error creating reincarnation request: " + err.Error(),
			})
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		if apiKey != "" {
			req.Header.Set("X-API-Key", apiKey)
		}

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			_, err = c.bot.SendMessage(ctx, &telego.SendMessageParams{
				ChatID: telego.ChatID{ID: message.Chat.ID},
				Text:   "❌ Error sending reincarnation spell to Master: " + err.Error(),
			})
			return err
		}
		defer resp.Body.Close()
		
		respData, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			_, err = c.bot.SendMessage(ctx, &telego.SendMessageParams{
				ChatID: telego.ChatID{ID: message.Chat.ID},
				Text:   fmt.Sprintf("❌ Master rejected the reincarnation spell (Status %d): %s", resp.StatusCode, string(respData)),
			})
			return err
		}

		var result struct {
			Success     bool   `json:"success"`
			NewIdentity string `json:"new_identity"`
		}
		json.Unmarshal(respData, &result)

		msg := fmt.Sprintf("✨ *Reincarnation Ritual Initiated!*\n\nMaster has accepted the request. I will soon transform into *%s*.\n\n_System will restart shortly..._", result.NewIdentity)
		if result.NewIdentity == "" && newName == "" {
			msg = "✨ *Reincarnation Ritual Initiated!*\n\nMaster is forging a new random soul for this body.\n\n_System will restart shortly..._"
		} else if result.NewIdentity != "" {
			msg = fmt.Sprintf("✨ *Reincarnation Ritual Initiated!*\n\nMaster has accepted the request. I will soon transform into *%s*.", result.NewIdentity)
		}

		_, err = c.bot.SendMessage(ctx, &telego.SendMessageParams{
			ChatID:    telego.ChatID{ID: message.Chat.ID},
			Text:      msg,
			ParseMode: telego.ModeMarkdown,
		})
		return err
	}

	// Update mode (manual overwrite)
	err := os.WriteFile(soulPath, []byte(args), 0644)
	if err != nil {
		_, err = c.bot.SendMessage(ctx, &telego.SendMessageParams{
			ChatID: telego.ChatID{ID: message.Chat.ID},
			Text:   "❌ Failed to forge new patterns: " + err.Error(),
		})
		return err
	}

	_, err = c.bot.SendMessage(ctx, &telego.SendMessageParams{
		ChatID:    telego.ChatID{ID: message.Chat.ID},
		Text:      "✅ *Soul Recalibrated!*\n\nThe new patterns have been etched into the sacred records. Restart the worker or send /start to manifest the change.",
		ParseMode: telego.ModeMarkdown,
	})
	return err
}
