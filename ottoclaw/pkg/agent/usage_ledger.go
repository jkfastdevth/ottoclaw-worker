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

// UsageLedger persists token usage data per workspace.
type UsageLedger struct {
	workspace  string
	ledgerFile string
	mu         sync.RWMutex
	Daily      map[string]int `json:"daily"` // date -> total_tokens
}

// NewUsageLedger creates a new usage ledger for the given workspace.
func NewUsageLedger(workspace string) *UsageLedger {
	ledgerFile := filepath.Join(workspace, "usage_ledger.json")
	l := &UsageLedger{
		workspace:  workspace,
		ledgerFile: ledgerFile,
		Daily:      make(map[string]int),
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
	return l.save()
}
