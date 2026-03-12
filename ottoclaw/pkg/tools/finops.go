package tools

import (
	"context"
	"fmt"
)

// UsageProvider is an interface for something that can provide token usage info.
type UsageProvider interface {
	GetTodayUsage() int
	GetMaxDailyTokens() int
}

// SiamTokenStatusTool allows an agent to check its own usage.
type SiamTokenStatusTool struct {
	Provider UsageProvider
}

func (t *SiamTokenStatusTool) Name() string { return "siam_token_status" }
func (t *SiamTokenStatusTool) Description() string {
	return "Check your current daily token usage, budget limit, and remaining quota."
}
func (t *SiamTokenStatusTool) Parameters() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (t *SiamTokenStatusTool) Execute(_ context.Context, _ map[string]any) *ToolResult {
	if t.Provider == nil {
		return ErrorResult("Usage provider not initialized")
	}

	used := t.Provider.GetTodayUsage()
	max := t.Provider.GetMaxDailyTokens()

	if max <= 0 {
		return UserResult(fmt.Sprintf("Daily Usage: %d tokens. (No daily limit set)", used))
	}

	percent := float64(used) / float64(max) * 100
	remaining := max - used
	if remaining < 0 {
		remaining = 0
	}

	return UserResult(fmt.Sprintf("Daily Usage: %d / %d tokens (%.1f%% used). Remaining: %d tokens.", 
		used, max, percent, remaining))
}

// AgentUsageStats holds usage information for an agent, defined in tools to avoid circular dependency.
type AgentUsageStats struct {
	AgentID        string `json:"agent_id"`
	TodayUsage     int    `json:"today_usage"`
	MaxDailyTokens int    `json:"max_daily_tokens"`
}

// UsageRegistry is an interface for something that can provide usage stats for all agents.
type UsageRegistry interface {
	SummarizeUsage() []AgentUsageStats
}

// SiamFinOpsReportTool provides a summary of usage for all local agents.
type SiamFinOpsReportTool struct {
	Registry UsageRegistry
}

func (t *SiamFinOpsReportTool) Name() string { return "siam_finops_report" }
func (t *SiamFinOpsReportTool) Description() string {
	return "Generate a FinOps report summarizing token usage and budget status for all active agents in the office."
}
func (t *SiamFinOpsReportTool) Parameters() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (t *SiamFinOpsReportTool) Execute(_ context.Context, _ map[string]any) *ToolResult {
	if t.Registry == nil {
		return ErrorResult("Usage registry not initialized")
	}

	stats := t.Registry.SummarizeUsage()
	if len(stats) == 0 {
		return UserResult("FinOps Report: No active agent data found.")
	}

	report := "💰 **Siam-Synapse FinOps: Daily Token Usage Report** 📊\n\n"
	report += "| Agent ID | Usage | Limit | Status |\n"
	report += "| :--- | :--- | :--- | :--- |\n"

	for _, s := range stats {
		status := "✅ OK"
		if s.MaxDailyTokens > 0 {
			percent := float64(s.TodayUsage) / float64(s.MaxDailyTokens) * 100
			if percent >= 100 {
				status = "🔴 OVER BUDGET"
			} else if percent >= 80 {
				status = "⚠️ WARNING"
			}
		}

		limitStr := "Unlimited"
		if s.MaxDailyTokens > 0 {
			limitStr = fmt.Sprintf("%d", s.MaxDailyTokens)
		}

		report += fmt.Sprintf("| %s | %d | %s | %s |\n", s.AgentID, s.TodayUsage, limitStr, status)
	}

	return UserResult(report)
}
