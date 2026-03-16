package utils

import (
	"os"
	"testing"
)

func TestGetEffectiveProxy(t *testing.T) {
	// Test env override
	os.Setenv("OTTOCLAW_PROXY", "http://env-proxy:8080")
	defer os.Unsetenv("OTTOCLAW_PROXY")

	p := GetEffectiveProxy("http://explicit-proxy:8080")
	if p != "http://env-proxy:8080" {
		t.Errorf("Expected env proxy override, got %q", p)
	}

	// Test fallback to explicit
	os.Unsetenv("OTTOCLAW_PROXY")
	p = GetEffectiveProxy("http://explicit-proxy:8080")
	if p != "http://explicit-proxy:8080" {
		t.Errorf("Expected explicit proxy, got %q", p)
	}

	// Test empty all
	p = GetEffectiveProxy("")
	if p != "" {
		t.Errorf("Expected empty proxy, got %q", p)
	}
}
