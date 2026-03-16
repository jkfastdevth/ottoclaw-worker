package utils

import (
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	// Set RPM to 60 (1 request per second)
	rl := NewRateLimiter(60)

	start := time.Now()
	rl.Wait() // First call should be instant
	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Errorf("First Wait() took too long: %v", elapsed)
	}

	start = time.Now()
	rl.Wait() // Second call should take about 1 second
	elapsed = time.Since(start)
	if elapsed < 800*time.Millisecond {
		t.Errorf("Second Wait() was too fast: %v", elapsed)
	}
}

func TestRateLimiterDisabled(t *testing.T) {
	rl := NewRateLimiter(0) // Disabled

	start := time.Now()
	for i := 0; i < 10; i++ {
		rl.Wait()
	}
	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Errorf("Disabled limiter should not block, but took %v", elapsed)
	}
}
