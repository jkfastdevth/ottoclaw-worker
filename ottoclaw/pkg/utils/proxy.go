package utils

import (
	"os"
)

// GetEffectiveProxy returns the proxy URL to use.
// It prioritizes OTTOCLAW_PROXY environment variable, then the explicitly provided proxy.
func GetEffectiveProxy(explicitProxy string) string {
	if p := os.Getenv("OTTOCLAW_PROXY"); p != "" {
		return p
	}
	return explicitProxy
}
