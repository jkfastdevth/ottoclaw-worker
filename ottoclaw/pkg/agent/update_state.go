package agent

import "sync"

var (
	updateStatus string = "idle" // "idle", "failed"
	updateError  string
	updateMu     sync.Mutex
)

// SetUpdateStatus records the outcome of a self-update attempt.
func SetUpdateStatus(status, errStr string) {
	updateMu.Lock()
	defer updateMu.Unlock()
	updateStatus = status
	updateError = errStr
}

// GetUpdateStatus returns current update status and error log.
func GetUpdateStatus() (string, string) {
	updateMu.Lock()
	defer updateMu.Unlock()
	return updateStatus, updateError
}

// ClearUpdateStatus resets status after it has been reported to Master.
func ClearUpdateStatus() {
	updateMu.Lock()
	defer updateMu.Unlock()
	updateStatus = "idle"
	updateError = ""
}
