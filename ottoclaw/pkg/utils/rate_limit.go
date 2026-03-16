package utils

import (
	"sync"
	"time"
)

// RateLimiter implements a simple thread-safe rate limiter.
type RateLimiter struct {
	mu           sync.Mutex
	rpm          int
	tokens       float64
	lastUpdate   time.Time
}

// NewRateLimiter creates a new rate limiter with the specified Requests Per Minute.
func NewRateLimiter(rpm int) *RateLimiter {
	return &RateLimiter{
		rpm:        rpm,
		tokens:     1.0,
		lastUpdate: time.Now(),
	}
}

// Wait blocks until a request can be made, or until the context is canceled.
func (rl *RateLimiter) Wait() {
	if rl.rpm <= 0 {
		return
	}

	for {
		rl.mu.Lock()
		now := time.Now()
		elapsed := now.Sub(rl.lastUpdate).Seconds()
		rl.lastUpdate = now

		// Refill tokens
		rl.tokens += elapsed * (float64(rl.rpm) / 60.0)
		if rl.tokens > float64(rl.rpm) {
			rl.tokens = float64(rl.rpm)
		}

		if rl.tokens >= 1.0 {
			rl.tokens -= 1.0
			rl.mu.Unlock()
			return
		}

		rl.mu.Unlock()
		// Wait for token to refill (approximate)
		time.Sleep(100 * time.Millisecond)
	}
}

// Global limiter map for model-based limiting
var (
	limiters   = make(map[string]*RateLimiter)
	limitersMu sync.RWMutex
)

// GetLimiter returns a rate limiter for a specific model/key combination.
func GetLimiter(key string, rpm int) *RateLimiter {
	limitersMu.RLock()
	rl, ok := limiters[key]
	limitersMu.RUnlock()

	if ok {
		return rl
	}

	limitersMu.Lock()
	defer limitersMu.Unlock()

	// Double check
	if rl, ok = limiters[key]; ok {
		return rl
	}

	rl = NewRateLimiter(rpm)
	limiters[key] = rl
	return rl
}
