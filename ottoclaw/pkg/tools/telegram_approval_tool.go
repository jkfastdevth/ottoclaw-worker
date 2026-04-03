package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sipeed/ottoclaw/pkg/logger"
)

// TelegramApprovalTool pauses the execution of an agent step and waits for human approval via Telegram.
type TelegramApprovalTool struct{}

func NewTelegramApprovalTool() *TelegramApprovalTool {
	return &TelegramApprovalTool{}
}

func (t *TelegramApprovalTool) Name() string {
	return "telegram_request_approval"
}

func (t *TelegramApprovalTool) Description() string {
	return "Ask the human admin for approval via Telegram and PAUSE execution until they type 'approve' or 'reject'. " +
		"Use this before taking risky or observable action (like posting to Facebook). " +
		"If the human rejects, this tool returns an error."
}

func (t *TelegramApprovalTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"message": map[string]any{
				"type":        "string",
				"description": "The exact question / context to send to the admin to evaluate before approving.",
			},
			"timeout_minutes": map[string]any{
				"type":        "integer",
				"description": "How many minutes to wait before timing out. Default is 5.",
			},
			"chat_id": map[string]any{
				"type":        "string",
				"description": "Telegram chat ID. If omitted, uses environment variables.",
			},
		},
		"required": []string{"message"},
	}
}

func (t *TelegramApprovalTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	message, _ := args["message"].(string)
	if message == "" {
		return ErrorResult("message is required")
	}

	timeoutMinutes := 5
	if val, ok := args["timeout_minutes"].(float64); ok && val > 0 {
		timeoutMinutes = int(val)
	} else if valStr, ok := args["timeout_minutes"].(string); ok {
		importStrconv := false
		_ = importStrconv // we need to import strconv, I will do it next
		var v int
		fmt.Sscanf(valStr, "%d", &v)
		if v > 0 {
			timeoutMinutes = v
		}
	}

	token := os.Getenv("ORCHESTRATOR_TELEGRAM_TOKEN")
	if token == "" {
		token = os.Getenv("TELEGRAM_BOT_TOKEN")
	}
	if token == "" {
		return ErrorResult("TELEGRAM_BOT_TOKEN not set in environment")
	}

	chatID, _ := args["chat_id"].(string)
	if chatID == "" {
		chatID = os.Getenv("TELEGRAM_BRIDGE_CHAT_ID")
	}
	if chatID == "" {
		chatID = os.Getenv("TELEGRAM_ADMIN_CHAT_ID")
	}
	if chatID == "" {
		return ErrorResult("chat_id not provided and TELEGRAM_ADMIN_CHAT_ID / TELEGRAM_BRIDGE_CHAT_ID not set")
	}

	// 1. Flush existing updates to find curOffset (so we ignore old "approve" messages)
	curOffset := 0
	getUpdatesURL := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates", token)
	resp, err := http.Get(getUpdatesURL)
	if err == nil {
		defer resp.Body.Close()
		var guResp struct {
			OK     bool `json:"ok"`
			Result []struct {
				UpdateID int `json:"update_id"`
			} `json:"result"`
		}
		if json.NewDecoder(resp.Body).Decode(&guResp) == nil && guResp.OK {
			for _, u := range guResp.Result {
				if u.UpdateID >= curOffset {
					curOffset = u.UpdateID + 1
				}
			}
		}
	}

	// 2. Send the question
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	
	payloadMsg := fmt.Sprintf("🛡️ <b>[Approval Required]</b>\n\n%s", message)
	
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       payloadMsg,
		"parse_mode": "HTML",
		"reply_markup": map[string]interface{}{
			"inline_keyboard": [][]map[string]interface{}{
				{
					{"text": "✅ Approve", "callback_data": "approve"},
					{"text": "❌ Reject", "callback_data": "reject"},
				},
			},
		},
	}
	bodyData, _ := json.Marshal(payload)
	
	reqMsg, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(bodyData))
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to create request: %v", err))
	}
	reqMsg.Header.Set("Content-Type", "application/json")
	
	sendResp, err := http.DefaultClient.Do(reqMsg)
	if err != nil || sendResp.StatusCode != 200 {
		return ErrorResult(fmt.Sprintf("Failed to send approval message: %v", err))
	}
	sendResp.Body.Close()

	// 3. Enter Polling Loop for the reply
	logger.InfoCF("tool", "Waiting for human approval via Telegram", map[string]interface{}{"timeout_min": timeoutMinutes})
	
	deadline := time.Now().Add(time.Duration(timeoutMinutes) * time.Minute)
	
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ErrorResult("Context canceled while waiting for approval")
		default:
			// Poll Telegram
			pollURL := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%d&timeout=5", token, curOffset)
			tResp, err := http.Get(pollURL)
			if err != nil {
				time.Sleep(2 * time.Second)
				continue
			}

			var guResp struct {
				OK     bool `json:"ok"`
				Result []struct {
					UpdateID int `json:"update_id"`
					Message  *struct {
						Chat struct {
							ID json.Number `json:"id"`
						} `json:"chat"`
						Text string `json:"text"`
					} `json:"message"`
					CallbackQuery *struct {
						ID   string `json:"id"`
						Data string `json:"data"`
						Message *struct {
							Chat struct {
								ID json.Number `json:"id"`
							} `json:"chat"`
							MessageID int `json:"message_id"`
						} `json:"message"`
					} `json:"callback_query"`
				} `json:"result"`
			}
			json.NewDecoder(tResp.Body).Decode(&guResp)
			tResp.Body.Close()

			if guResp.OK {
				for _, update := range guResp.Result {
					curOffset = update.UpdateID + 1

					var activeChatID string
					var textLower string
					var callbackID string
					var messageID int

					if update.CallbackQuery != nil && update.CallbackQuery.Message != nil {
						activeChatID = update.CallbackQuery.Message.Chat.ID.String()
						textLower = strings.ToLower(strings.TrimSpace(update.CallbackQuery.Data))
						callbackID = update.CallbackQuery.ID
						messageID = update.CallbackQuery.Message.MessageID
					} else if update.Message != nil {
						activeChatID = update.Message.Chat.ID.String()
						textLower = strings.ToLower(strings.TrimSpace(update.Message.Text))
					} else {
						continue
					}

					// Validate chat matching
					if activeChatID != chatID {
						continue
					}

					// Acknowledge callback query if present to stop loading spinner
					if callbackID != "" {
						cbURL := fmt.Sprintf("https://api.telegram.org/bot%s/answerCallbackQuery", token)
						cbBody, _ := json.Marshal(map[string]interface{}{"callback_query_id": callbackID})
						http.Post(cbURL, "application/json", bytes.NewBuffer(cbBody))
					}

					if textLower == "approve" || textLower == "yes" || textLower == "y" {
						// Send confirmation
						confirmPayload := map[string]interface{}{
							"chat_id": chatID,
							"text":    "✅ <b>Approved.</b> Resuming execution...",
							"parse_mode": "HTML",
						}
						cbd, _ := json.Marshal(confirmPayload)
						cReq, _ := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(cbd))
						cReq.Header.Set("Content-Type", "application/json")
						cResp, _ := http.DefaultClient.Do(cReq)
						if cResp != nil {
							cResp.Body.Close()
						}
						
						// Remove inline keyboard from original message if it was a button click
						if messageID != 0 {
							editURL := fmt.Sprintf("https://api.telegram.org/bot%s/editMessageReplyMarkup", token)
							editBody, _ := json.Marshal(map[string]interface{}{
								"chat_id": chatID,
								"message_id": messageID,
								"reply_markup": map[string]interface{}{"inline_keyboard": [][]interface{}{}},
							})
							http.Post(editURL, "application/json", bytes.NewBuffer(editBody))
						}

						return &ToolResult{
							ForLLM:  "User has APPROVED the action. Please proceed. Your execution has resumed.",
							ForUser: "Approved by human admin.",
						}
					} else if textLower == "reject" || textLower == "cancel" || textLower == "no" || textLower == "n" || textLower == "abort" {
						// Send rejection confirmation
						confirmPayload := map[string]interface{}{
							"chat_id": chatID,
							"text":    "❌ <b>Rejected.</b> Aborting task...",
							"parse_mode": "HTML",
						}
						cbd, _ := json.Marshal(confirmPayload)
						cReq, _ := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(cbd))
						cReq.Header.Set("Content-Type", "application/json")
						cResp, _ := http.DefaultClient.Do(cReq)
						if cResp != nil {
							cResp.Body.Close()
						}

						if messageID != 0 {
							editURL := fmt.Sprintf("https://api.telegram.org/bot%s/editMessageReplyMarkup", token)
							editBody, _ := json.Marshal(map[string]interface{}{
								"chat_id": chatID,
								"message_id": messageID,
								"reply_markup": map[string]interface{}{"inline_keyboard": [][]interface{}{}},
							})
							http.Post(editURL, "application/json", bytes.NewBuffer(editBody))
						}
						
						return ErrorResult("User REJECTED the action. STOP processing and abort.")
					}
				}
			}
			
			time.Sleep(1 * time.Second)
		}
	}

	return ErrorResult("Timed out waiting for human approval")
}
