package main

// proactive.go — Phase D: Proactive Agent (Scheduled + Event-Driven Actions)
//
// Enables workers to act autonomously on schedules or device events.
// Triggers are stored in memory and survive across command cycles (not restarts).
//
// Built-in trigger types:
//   interval   — run every N seconds (e.g. "3600" = hourly)
//   cron       — simplified hh:mm daily schedule (e.g. "08:00")
//   battery_low — fires when battery drops below threshold (default 20%)
//
// Each trigger sends a SYSTEM_BRAIN_QUERY to the Control Brain with a prompt
// built from the trigger's action + latest hardware snapshot.
//
// Commands (from master → worker):
//   SYSTEM_PROACTIVE_ADD    — JSON ProactiveTrigger
//   SYSTEM_PROACTIVE_REMOVE — trigger ID string
//   SYSTEM_PROACTIVE_LIST   — no payload, returns JSON array
//   SYSTEM_PROACTIVE_RUN    — trigger ID — run immediately

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// ─── Data Types ───────────────────────────────────────────────────────────────

// ProactiveTrigger defines a scheduled or event-driven action for the agent.
type ProactiveTrigger struct {
	ID          string `json:"id"`           // unique slug (e.g. "morning-brief")
	Name        string `json:"name"`         // human label
	Type        string `json:"type"`         // "interval" | "cron" | "battery_low"
	Schedule    string `json:"schedule"`     // interval: seconds; cron: "HH:MM"; battery_low: threshold %
	Action      string `json:"action"`       // prompt template sent to brain (use {battery}, {time}, etc.)
	Enabled     bool   `json:"enabled"`
	LastFiredAt *time.Time `json:"last_fired_at,omitempty"`
}

// ─── Global Trigger Registry ─────────────────────────────────────────────────

var (
	proactiveTriggers   = map[string]*ProactiveTrigger{}
	proactiveMu         sync.RWMutex
	proactiveLoopCancel context.CancelFunc
)

// ─── Built-in Triggers ────────────────────────────────────────────────────────

func defaultTriggers() []*ProactiveTrigger {
	return []*ProactiveTrigger{
		{
			ID:       "morning-brief",
			Name:     "Morning Briefing",
			Type:     "cron",
			Schedule: "08:00",
			Action:   "Good morning! Please give a brief status overview: battery level, any important tasks pending, and a motivational message.",
			Enabled:  true,
		},
		{
			ID:       "battery-low-alert",
			Name:     "Battery Low Alert",
			Type:     "battery_low",
			Schedule: "20", // fire when battery <= 20%
			Action:   "Battery is running low ({battery}%). Please alert the user and suggest plugging in the charger.",
			Enabled:  true,
		},
		{
			ID:       "memory-remind",
			Name:     "Daily Memory Reminder",
			Type:     "cron",
			Schedule: "21:00",
			Action:   "Evening check-in: summarize what was accomplished today based on your memory and suggest anything to remember for tomorrow.",
			Enabled:  false, // opt-in
		},
	}
}

// ─── Lifecycle ───────────────────────────────────────────────────────────────

// StartProactiveLoop initialises built-in triggers and begins the evaluation loop.
func StartProactiveLoop(parentCtx context.Context) {
	proactiveMu.Lock()
	// Load built-ins if registry is empty
	if len(proactiveTriggers) == 0 {
		for _, t := range defaultTriggers() {
			proactiveTriggers[t.ID] = t
		}
	}
	proactiveMu.Unlock()

	ctx, cancel := context.WithCancel(parentCtx)
	proactiveLoopCancel = cancel

	go runProactiveLoop(ctx)
	log.Println("🤖 [Proactive] Loop started")
}

// StopProactiveLoop cancels the evaluation loop.
func StopProactiveLoop() {
	if proactiveLoopCancel != nil {
		proactiveLoopCancel()
		proactiveLoopCancel = nil
		log.Println("🤖 [Proactive] Loop stopped")
	}
}

// runProactiveLoop ticks every 30 seconds and evaluates all triggers.
func runProactiveLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Also run immediately on startup (after a short delay for brain to warm up)
	warmup := time.NewTimer(10 * time.Second)
	defer warmup.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-warmup.C:
			evaluateAllTriggers(ctx)
		case <-ticker.C:
			evaluateAllTriggers(ctx)
		}
	}
}

// ─── Trigger Evaluation ───────────────────────────────────────────────────────

func evaluateAllTriggers(ctx context.Context) {
	proactiveMu.RLock()
	triggers := make([]*ProactiveTrigger, 0, len(proactiveTriggers))
	for _, t := range proactiveTriggers {
		if t.Enabled {
			triggers = append(triggers, t)
		}
	}
	proactiveMu.RUnlock()

	for _, t := range triggers {
		if shouldFire(ctx, t) {
			go fireTrigger(ctx, t)
		}
	}
}

// shouldFire returns true if a trigger is due to fire now.
func shouldFire(ctx context.Context, t *ProactiveTrigger) bool {
	now := time.Now()

	switch t.Type {
	case "interval":
		// Parse schedule as seconds
		var secs int
		fmt.Sscanf(t.Schedule, "%d", &secs)
		if secs <= 0 {
			return false
		}
		if t.LastFiredAt == nil {
			return true
		}
		return now.Sub(*t.LastFiredAt) >= time.Duration(secs)*time.Second

	case "cron":
		// Schedule = "HH:MM" in Asia/Bangkok
		loc, _ := time.LoadLocation("Asia/Bangkok")
		nowLocal := now.In(loc)
		parts := strings.SplitN(t.Schedule, ":", 2)
		if len(parts) != 2 {
			return false
		}
		var h, m int
		fmt.Sscanf(parts[0], "%d", &h)
		fmt.Sscanf(parts[1], "%d", &m)
		// Fire if we're within the current minute window and haven't fired today
		if nowLocal.Hour() == h && nowLocal.Minute() == m {
			if t.LastFiredAt == nil {
				return true
			}
			// Prevent double-fire within same minute
			return now.Sub(*t.LastFiredAt) >= 60*time.Second
		}
		return false

	case "battery_low":
		// Schedule = threshold percentage string
		var threshold float64
		threshold = 20 // default
		fmt.Sscanf(t.Schedule, "%f", &threshold)

		bat, err := GetBattery(ctx)
		if err != nil || bat == nil {
			return false
		}
		if bat.Plugged {
			return false // charging — no need to alert
		}
		if bat.Level > threshold {
			return false
		}
		// Throttle: only fire once per 30 minutes
		if t.LastFiredAt != nil && time.Since(*t.LastFiredAt) < 30*time.Minute {
			return false
		}
		return true
	}
	return false
}

// fireTrigger executes a trigger — builds the brain prompt and dispatches it.
func fireTrigger(ctx context.Context, t *ProactiveTrigger) {
	now := time.Now()

	// Mark fired immediately to prevent duplicate fires
	proactiveMu.Lock()
	t.LastFiredAt = &now
	proactiveMu.Unlock()

	log.Printf("🤖 [Proactive] Firing trigger: %s (%s)", t.ID, t.Name)

	// Expand placeholders in action prompt
	prompt := expandTriggerPrompt(ctx, t.Action)

	// Send to Control Brain via HandleBrainQuery
	result := HandleBrainQuery(ctx, prompt)
	log.Printf("🤖 [Proactive] Trigger %s result: %.120s", t.ID, result)
}

// expandTriggerPrompt replaces {battery}, {time}, {date} in the action string.
func expandTriggerPrompt(ctx context.Context, action string) string {
	loc, _ := time.LoadLocation("Asia/Bangkok")
	now := time.Now().In(loc)

	action = strings.ReplaceAll(action, "{time}", now.Format("15:04"))
	action = strings.ReplaceAll(action, "{date}", now.Format("2006-01-02"))
	action = strings.ReplaceAll(action, "{day}", now.Weekday().String())

	// Battery placeholder
	if strings.Contains(action, "{battery}") {
		if bat, err := GetBattery(ctx); err == nil && bat != nil {
			action = strings.ReplaceAll(action, "{battery}", fmt.Sprintf("%.0f", bat.Level))
		} else {
			action = strings.ReplaceAll(action, "{battery}", "unknown")
		}
	}

	return action
}

// ─── SYSTEM_PROACTIVE_* Command Handlers ─────────────────────────────────────

// HandleProactiveAdd adds or replaces a trigger from JSON payload.
func HandleProactiveAdd(payload string) string {
	var t ProactiveTrigger
	if err := json.Unmarshal([]byte(payload), &t); err != nil {
		return fmt.Sprintf(`{"error":"invalid trigger JSON: %s"}`, err.Error())
	}
	if t.ID == "" {
		return `{"error":"trigger id is required"}`
	}
	if t.Type == "" {
		t.Type = "interval"
	}
	t.Enabled = true

	proactiveMu.Lock()
	proactiveTriggers[t.ID] = &t
	proactiveMu.Unlock()

	log.Printf("🤖 [Proactive] Added trigger: %s (%s)", t.ID, t.Name)
	out, _ := json.Marshal(map[string]interface{}{"success": true, "trigger": t})
	return string(out)
}

// HandleProactiveRemove removes a trigger by ID.
func HandleProactiveRemove(id string) string {
	id = strings.TrimSpace(id)
	proactiveMu.Lock()
	_, existed := proactiveTriggers[id]
	delete(proactiveTriggers, id)
	proactiveMu.Unlock()

	if !existed {
		return fmt.Sprintf(`{"error":"trigger %q not found"}`, id)
	}
	log.Printf("🤖 [Proactive] Removed trigger: %s", id)
	return fmt.Sprintf(`{"success":true,"removed":%q}`, id)
}

// HandleProactiveList returns all triggers as JSON.
func HandleProactiveList() string {
	proactiveMu.RLock()
	list := make([]*ProactiveTrigger, 0, len(proactiveTriggers))
	for _, t := range proactiveTriggers {
		list = append(list, t)
	}
	proactiveMu.RUnlock()

	out, _ := json.Marshal(map[string]interface{}{"triggers": list, "count": len(list)})
	return string(out)
}

// HandleProactiveRun fires a trigger immediately by ID.
func HandleProactiveRun(ctx context.Context, id string) string {
	id = strings.TrimSpace(id)

	proactiveMu.RLock()
	t, ok := proactiveTriggers[id]
	proactiveMu.RUnlock()

	if !ok {
		return fmt.Sprintf(`{"error":"trigger %q not found"}`, id)
	}
	go fireTrigger(ctx, t)
	return fmt.Sprintf(`{"success":true,"fired":%q}`, id)
}

// ─── Persistence (optional — file-based) ─────────────────────────────────────

func proactiveStatePath() string {
	home, _ := os.UserHomeDir()
	return home + "/.picoclaw/proactive_triggers.json"
}

// SaveProactiveTriggers persists current triggers to disk.
func SaveProactiveTriggers() {
	proactiveMu.RLock()
	data, err := json.MarshalIndent(proactiveTriggers, "", "  ")
	proactiveMu.RUnlock()
	if err != nil {
		return
	}
	os.WriteFile(proactiveStatePath(), data, 0600) //nolint:errcheck
}

// LoadProactiveTriggers restores triggers from disk (called at startup).
func LoadProactiveTriggers() {
	data, err := os.ReadFile(proactiveStatePath())
	if err != nil {
		return // first run — no file yet
	}
	var saved map[string]*ProactiveTrigger
	if err := json.Unmarshal(data, &saved); err != nil {
		return
	}
	proactiveMu.Lock()
	for id, t := range saved {
		proactiveTriggers[id] = t
	}
	proactiveMu.Unlock()
	log.Printf("🤖 [Proactive] Loaded %d triggers from disk", len(saved))
}
