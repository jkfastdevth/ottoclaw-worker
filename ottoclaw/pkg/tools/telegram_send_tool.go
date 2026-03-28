package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// TelegramSendTool uploads files (photos, documents) to Telegram via the Bot API.
// Uses ORCHESTRATOR_TELEGRAM_TOKEN from environment.
type TelegramSendTool struct{}

func NewTelegramSendTool() *TelegramSendTool {
	return &TelegramSendTool{}
}

func (t *TelegramSendTool) Name() string {
	return "telegram_send_file"
}

func (t *TelegramSendTool) Description() string {
	return "Send a local file (image, document, video) to a Telegram chat via the Bot API. " +
		"Uses ORCHESTRATOR_TELEGRAM_TOKEN from the environment. " +
		"For photos taken with termux_api camera-photo, pass the saved file path here to send it back to the user. " +
		"If chat_id is omitted, uses TELEGRAM_ADMIN_CHAT_ID or TELEGRAM_BRIDGE_CHAT_ID from environment."
}

func (t *TelegramSendTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "Absolute or relative path to the file to send (e.g. /tmp/photo.jpg)",
			},
			"chat_id": map[string]any{
				"type":        "string",
				"description": "Telegram chat ID to send to. If omitted, uses TELEGRAM_ADMIN_CHAT_ID or TELEGRAM_BRIDGE_CHAT_ID env var.",
			},
			"caption": map[string]any{
				"type":        "string",
				"description": "Optional caption for the file.",
			},
			"type": map[string]any{
				"type":        "string",
				"enum":        []string{"photo", "document"},
				"description": "Send as 'photo' (inline preview) or 'document' (original file). Default: photo for images, document otherwise.",
			},
		},
		"required": []string{"file_path"},
	}
}

func (t *TelegramSendTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	filePath, _ := args["file_path"].(string)
	if filePath == "" {
		return ErrorResult("file_path is required")
	}

	token := os.Getenv("ORCHESTRATOR_TELEGRAM_TOKEN")
	if token == "" {
		token = os.Getenv("TELEGRAM_BOT_TOKEN")
	}
	if token == "" {
		return ErrorResult("ORCHESTRATOR_TELEGRAM_TOKEN not set in environment")
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

	caption, _ := args["caption"].(string)
	sendType, _ := args["type"].(string)

	// Auto-detect type from extension if not specified
	if sendType == "" {
		ext := strings.ToLower(filepath.Ext(filePath))
		switch ext {
		case ".jpg", ".jpeg", ".png", ".gif", ".webp":
			sendType = "photo"
		default:
			sendType = "document"
		}
	}

	// Open the file
	f, err := os.Open(filePath)
	if err != nil {
		return ErrorResult(fmt.Sprintf("cannot open file: %v", err))
	}
	defer f.Close()

	// Build multipart form
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	_ = writer.WriteField("chat_id", chatID)
	if caption != "" {
		_ = writer.WriteField("caption", caption)
	}

	fieldName := sendType // "photo" or "document"
	part, err := writer.CreateFormFile(fieldName, filepath.Base(filePath))
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to create form field: %v", err))
	}
	if _, err = io.Copy(part, f); err != nil {
		return ErrorResult(fmt.Sprintf("failed to read file: %v", err))
	}
	writer.Close()

	// Choose API method
	method := "sendPhoto"
	if sendType == "document" {
		method = "sendDocument"
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/%s", token, method)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, &body)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to create request: %v", err))
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return ErrorResult(fmt.Sprintf("telegram API request failed: %v", err))
	}
	defer resp.Body.Close()

	var result struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ErrorResult(fmt.Sprintf("failed to decode response: %v", err))
	}
	if !result.OK {
		return ErrorResult(fmt.Sprintf("telegram API error: %s", result.Description))
	}

	msg := fmt.Sprintf("File sent successfully to chat %s via %s", chatID, method)
	return &ToolResult{
		ForLLM:  msg,
		ForUser: msg,
	}
}
