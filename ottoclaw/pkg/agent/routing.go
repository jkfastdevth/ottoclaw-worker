package agent

import "strings"

// parseRoutingHint extracts the routing profile from a [ROUTING:xxx] prefix in the message.
// Returns "" if no hint is present.
// Example: "[ROUTING:tool_calling] ถ่ายรูปด้วย termux_api..." → "tool_calling"
func parseRoutingHint(msg string) string {
	if !strings.HasPrefix(msg, "[ROUTING:") {
		return ""
	}
	end := strings.Index(msg, "]")
	if end < 0 {
		return ""
	}
	return strings.TrimPrefix(msg[:end], "[ROUTING:")
}

// routingModelName maps a routing profile to the model name entry in config.
// The model name must match a model_name entry in the agent's model_list.
//
// Profiles:
//
//	tool_calling — hardware/IO tasks, requires reliable function calling (Qwen3, Llama-4)
//	creative     — language/creative tasks, Thai-friendly models preferred
//	premium      — complex reasoning, best quality model
//	eco          — fast/cheap model, sufficient for simple confirmation tasks
func routingModelName(profile string) string {
	switch profile {
	case "tool_calling":
		return "tool_calling" // must match model_name in config.json model_list
	case "creative":
		return "creative"
	case "premium":
		return "premium"
	case "eco":
		return "eco"
	default:
		return ""
	}
}
