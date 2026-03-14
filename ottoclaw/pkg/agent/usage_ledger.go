package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sipeed/ottoclaw/pkg/fileutil"
	"github.com/sipeed/ottoclaw/pkg/providers"
)

// UsageDetail holds granular token usage information.
type UsageDetail struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// UsageLedger persists token usage data per workspace.
type UsageLedger struct {
	workspace    string
	ledgerFile   string
	mu           sync.RWMutex
	Daily        map[string]int         `json:"daily"`         // date -> total_tokens (legacy support)
	DailyDetails map[string]UsageDetail `json:"daily_details"` // date -> detailed usage
}

// NewUsageLedger creates a new usage ledger for the given workspace.
func NewUsageLedger(workspace string) *UsageLedger {
	ledgerFile := filepath.Join(workspace, "usage_ledger.json")
	l := &UsageLedger{
		workspace:    workspace,
		ledgerFile:   ledgerFile,
		Daily:        make(map[string]int),
		DailyDetails: make(map[string]UsageDetail),
	}
	l.load()
	return l
}

// load loads the ledger from disk.
func (l *UsageLedger) load() {
	l.mu.Lock()
	defer l.mu.Unlock()
	data, err := os.ReadFile(l.ledgerFile)
	if err == nil {
		json.Unmarshal(data, l)
	}
}

// save performs an atomic save of the ledger.
func (l *UsageLedger) save() error {
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	return fileutil.WriteFileAtomic(l.ledgerFile, data, 0o644)
}

// Record saves usage information to the ledger.
func (l *UsageLedger) Record(usage *providers.UsageInfo) error {
	if usage == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	l.Daily[today] += usage.TotalTokens

	detail := l.DailyDetails[today]
	detail.PromptTokens += usage.PromptTokens
	detail.CompletionTokens += usage.CompletionTokens
	detail.TotalTokens += usage.TotalTokens
	l.DailyDetails[today] = detail

	return l.save()
}

// GetTodayUsage returns the total tokens used today.
func (l *UsageLedger) GetTodayUsage() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	today := time.Now().Format("2006-01-02")
	return l.Daily[today]
}

// ResetTodayUsage resets the usage for today (useful for testing or manual overrides).
func (l *UsageLedger) ResetTodayUsage() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	today := time.Now().Format("2006-01-02")
	delete(l.Daily, today)
	delete(l.DailyDetails, today)
	return l.save()
}

// GetEstimatedCost handles basic cost calculation for today's usage.
// Note: This is a best-effort estimation based on average market rates.
// Rate: ~$0.01 per 1M tokens (blended average)
func (l *UsageLedger) GetEstimatedCost() float64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	today := time.Now().Format("2006-01-02")
	detail := l.DailyDetails[today]

	// Simple blended rate for now: $10 per 1M tokens ($0.00001 per token)
	// This is just a placeholder rate; in a real scenario, this would be model-specific.
	return float64(detail.TotalTokens) * 0.00001
}
