package utils

import (
	"regexp"
	"strings"
	"unicode"
)

func NormalizeID(id string) string {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	// Replace non-alphanumeric with dashes
	reg := regexp.MustCompile(`[^a-z0-9_-]+`)
	result := reg.ReplaceAllString(lower, "-")
	// Trim leading/trailing dashes
	result = strings.Trim(result, "-")
	return result
}

var (
	reThink    = regexp.MustCompile("(?s)<think>.*?</think>")
	reFunction = regexp.MustCompile("(?s)<function>.*?</function>")
)

// StripThinkTags removes the <think>...</think> sections from a string.
func StripThinkTags(input string) string {
	return StripInternalTags(input)
}

// StripInternalTags removes both <think> and <function> technical tags.
func StripInternalTags(input string) string {
	out := reThink.ReplaceAllString(input, "")
	out = reFunction.ReplaceAllString(out, "")
	return strings.TrimSpace(out)
}

// SanitizeMessageContent removes Unicode control characters, format characters (RTL overrides,
// zero-width characters), and other non-graphic characters that could confuse an LLM
// or cause display issues in the agent UI.
func SanitizeMessageContent(input string) string {
	var sb strings.Builder
	// Pre-allocate memory to avoid multiple allocations
	sb.Grow(len(input))

	for _, r := range input {
		// unicode.IsGraphic returns true if the rune is a Unicode graphic character.
		// This includes letters, marks, numbers, punctuation, and symbols.
		// It excludes control characters (Cc), format characters (Cf),
		// surrogates (Cs), and private use (Co).
		if unicode.IsGraphic(r) || r == '\n' || r == '\r' || r == '\t' {
			sb.WriteRune(r)
		}
	}

	return sb.String()
}

// Truncate returns a truncated version of s with at most maxLen runes.
// Handles multi-byte Unicode characters properly.
// If the string is truncated, "..." is appended to indicate truncation.
func Truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	// Reserve 3 chars for "..."
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

// DerefStr dereferences a pointer to a string and
// returns the value or a fallback if the pointer is nil.
func DerefStr(s *string, fallback string) string {
	if s == nil {
		return fallback
	}
	return *s
}
