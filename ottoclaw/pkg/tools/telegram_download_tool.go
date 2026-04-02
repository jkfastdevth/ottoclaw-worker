package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// TelegramDownloadTool retrieves the most recent photo sent to a Telegram chat.
// Uses ORCHESTRATOR_TELEGRAM_TOKEN from environment.
type TelegramDownloadTool struct{}

func NewTelegramDownloadTool() *TelegramDownloadTool {
	return &TelegramDownloadTool{}
}

func (t *TelegramDownloadTool) Name() string {
	return "telegram_download_latest_photo"
}

func (t *TelegramDownloadTool) Description() string {
	return "Download the most recent photo sent to a Telegram group/chat. " +
		"If chat_id is omitted, uses TELEGRAM_ADMIN_CHAT_ID or TELEGRAM_BRIDGE_CHAT_ID. " +
		"Saves to the specified save_path (default: /tmp/latest_telegram_photo.jpg)."
}

func (t *TelegramDownloadTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"save_path": map[string]any{
				"type":        "string",
				"description": "Absolute path to save the downloaded photo to. Default is /tmp/latest_telegram_photo.jpg",
			},
			"chat_id": map[string]any{
				"type":        "string",
				"description": "Telegram chat ID to search in. If omitted, uses environment variables.",
			},
		},
	}
}

func (t *TelegramDownloadTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	savePath, _ := args["save_path"].(string)
	if savePath == "" {
		savePath = "/tmp/latest_telegram_photo.jpg"
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

	// 1. Get Updates
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates", token)
	resp, err := http.Get(apiURL)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to getUpdates: %v", err))
	}
	defer resp.Body.Close()

	var getUpdatesResp struct {
		OK     bool `json:"ok"`
		Result []struct {
			Message struct {
				Chat struct {
					ID json.Number `json:"id"`
				} `json:"chat"`
				Photo []struct {
					FileID   string `json:"file_id"`
					FileSize int    `json:"file_size"`
				} `json:"photo"`
			} `json:"message"`
		} `json:"result"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&getUpdatesResp); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to decode getUpdates: %v", err))
	}

	if !getUpdatesResp.OK {
		return ErrorResult("getUpdates returned not OK. Ensure bot has access to messages or webhook is not blocking it.")
	}

	var bestFileID string
	var maxFileSize int

	// Loop backwards to find the latest photo in the required chat
	for i := len(getUpdatesResp.Result) - 1; i >= 0; i-- {
		update := getUpdatesResp.Result[i]
		
		// If chatID is provided, verify it matches
		if chatID != "" {
			if update.Message.Chat.ID.String() != chatID {
				continue
			}
		}

		photos := update.Message.Photo
		if len(photos) > 0 {
			// Telegram provides multiple sizes; pick the largest
			for _, p := range photos {
				if p.FileSize > maxFileSize || bestFileID == "" {
					maxFileSize = p.FileSize
					bestFileID = p.FileID
				}
			}
			break // Found the latest message with a photo
		}
	}

	if bestFileID == "" {
		return ErrorResult("No recent photo found in the Telegram chat history.")
	}

	// 2. Get File Path
	getFileURL := fmt.Sprintf("https://api.telegram.org/bot%s/getFile?file_id=%s", token, bestFileID)
	fResp, err := http.Get(getFileURL)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to getFile: %v", err))
	}
	defer fResp.Body.Close()

	var getFileResp struct {
		OK     bool `json:"ok"`
		Result struct {
			FilePath string `json:"file_path"`
		} `json:"result"`
	}
	
	if err := json.NewDecoder(fResp.Body).Decode(&getFileResp); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to decode getFile: %v", err))
	}

	if !getFileResp.OK || getFileResp.Result.FilePath == "" {
		return ErrorResult("Failed to retrieve file path from Telegram API.")
	}

	// 3. Download the actual image file
	downloadURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", token, getFileResp.Result.FilePath)
	imgResp, err := http.Get(downloadURL)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to download image: %v", err))
	}
	defer imgResp.Body.Close()

	outFile, err := os.Create(savePath)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to create local file: %v", err))
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, imgResp.Body); err != nil {
		return ErrorResult(fmt.Sprintf("Error writing to file: %v", err))
	}

	msg := fmt.Sprintf("Successfully downloaded the latest photo to %s", savePath)
	return &ToolResult{
		ForLLM:  msg,
		ForUser: msg,
	}
}
