package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// SiamBIReportTool ขอให้ master สร้าง Business Intelligence report
// ครอบคลุม CRM, FinOps, Missions แล้ว LLM สังเคราะห์เป็นภาษาไทย
type SiamBIReportTool struct{}

func (t *SiamBIReportTool) Name() string { return "siam_bi_report" }
func (t *SiamBIReportTool) Description() string {
	return "Generate a Business Intelligence report from CRM, FinOps, and mission data. LLM synthesizes the data into actionable Thai-language insights. Types: crm, finops, missions, strategy, full."
}

func (t *SiamBIReportTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"type": map[string]any{
				"type":        "string",
				"enum":        []string{"crm", "finops", "missions", "strategy", "full"},
				"description": "Report type: crm=customer analytics, finops=token costs, missions=operation stats, strategy=strategic recommendations, full=all combined",
			},
			"days": map[string]any{
				"type":        "integer",
				"description": "Lookback period in days (default: 7)",
			},
			"org_id": map[string]any{
				"type":        "string",
				"description": "Filter by organization ID (optional)",
			},
		},
		"required": []string{"type"},
	}
}

func (t *SiamBIReportTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	masterURL := os.Getenv("MASTER_URL")
	if masterURL == "" {
		masterURL = "http://master:8080"
	}
	apiKey := os.Getenv("MASTER_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("SIAM_API_KEY")
	}

	reportType := fmt.Sprintf("%v", args["type"])
	if reportType == "" || reportType == "<nil>" {
		reportType = "full"
	}

	days := 7
	if d, ok := args["days"]; ok {
		switch v := d.(type) {
		case float64:
			days = int(v)
		case int:
			days = v
		}
	}

	payload := map[string]any{
		"type": reportType,
		"days": days,
	}
	if orgID, ok := args["org_id"]; ok && fmt.Sprintf("%v", orgID) != "" && fmt.Sprintf("%v", orgID) != "<nil>" {
		payload["org_id"] = fmt.Sprintf("%v", orgID)
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", masterURL+"/api/agent/v1/bi/report", bytes.NewReader(body))
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to create request: %v", err))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ErrorResult(fmt.Sprintf("BI report request failed: %v", err))
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to parse response: %v", err))
	}
	if resp.StatusCode != 200 {
		return ErrorResult(fmt.Sprintf("BI report error (%d): %v", resp.StatusCode, result["error"]))
	}

	report := fmt.Sprintf("%v", result["report"])
	if report == "<nil>" || report == "" {
		report = fmt.Sprintf("%v", result["raw_data"])
	}

	return UserResult(fmt.Sprintf("📊 **BI Report (%s, %dd)**\n\n%s", reportType, days, report))
}

// SiamDailyRitualTool สั่งให้ master รัน daily ritual analysis ทันที
type SiamDailyRitualTool struct{}

func (t *SiamDailyRitualTool) Name() string { return "siam_daily_ritual" }
func (t *SiamDailyRitualTool) Description() string {
	return "Trigger the daily ritual cron manually — analyzes yesterday's missions, LLM usage, and CRM data, saves a report, and sends a Telegram summary."
}

func (t *SiamDailyRitualTool) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *SiamDailyRitualTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	masterURL := os.Getenv("MASTER_URL")
	if masterURL == "" {
		masterURL = "http://master:8080"
	}
	apiKey := os.Getenv("MASTER_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("SIAM_API_KEY")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", masterURL+"/api/agent/v1/ritual-cron/run", nil)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to create request: %v", err))
	}
	req.Header.Set("X-API-Key", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to trigger ritual: %v", err))
	}
	defer resp.Body.Close()

	return UserResult("🌙 Daily ritual started — analysis is running in background. Report will be sent to Telegram when complete.")
}
