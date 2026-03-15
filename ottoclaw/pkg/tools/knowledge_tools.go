package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// --- siam_learn Tool ---

// LearnTool lets an agent contribute knowledge to the Swarm's Akashic Library (Supabase RAG)
type LearnTool struct {
	MasterURL string
	APIKey    string
}

func NewLearnTool(masterURL, apiKey string) *LearnTool {
	if masterURL == "" {
		masterURL = os.Getenv("MASTER_API_URL")
		if masterURL == "" {
			masterURL = "http://master:8080"
		}
	}
	if apiKey == "" {
		apiKey = os.Getenv("MASTER_API_KEY")
	}
	return &LearnTool{MasterURL: masterURL, APIKey: apiKey}
}

func (t *LearnTool) Name() string { return "siam_learn" }

func (t *LearnTool) Description() string {
	return "Store a new piece of knowledge or fact into the Swarm's shared Akashic Library. Use this to remember important findings, facts, or insights that other agents should know about. Knowledge is embedded as a semantic vector for intelligent recall."
}

func (t *LearnTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"fact": map[string]any{
				"type":        "string",
				"description": "The piece of knowledge, fact, or insight to store. Should be a clear, self-contained statement.",
			},
			"confidence": map[string]any{
				"type":        "number",
				"description": "Your confidence in this fact, from 0.0 to 1.0 (default 1.0)",
			},
			"tags": map[string]any{
				"type":        "string",
				"description": "Comma-separated tags to categorize this knowledge (e.g., 'finance,crypto,market')",
			},
		},
		"required": []string{"fact"},
	}
}

func (t *LearnTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	fact, ok := args["fact"].(string)
	if !ok || strings.TrimSpace(fact) == "" {
		return ErrorResult("'fact' is required and must be a non-empty string")
	}

	agentID := ToolAgentID(ctx)
	confidence := 1.0
	if c, ok := args["confidence"].(float64); ok {
		confidence = c
	}

	var tags []string
	if ts, ok := args["tags"].(string); ok && ts != "" {
		for _, t := range strings.Split(ts, ",") {
			if trimmed := strings.TrimSpace(t); trimmed != "" {
				tags = append(tags, trimmed)
			}
		}
	}

	payload := map[string]any{
		"fact":         fact,
		"confidence":   confidence,
		"source_agent": agentID,
		"tags":         tags,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/api/agent/v1/knowledge", t.MasterURL),
		bytes.NewReader(body),
	)
	if err != nil {
		return ErrorResult("failed to build request: " + err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	if t.APIKey != "" {
		req.Header.Set("X-API-Key", t.APIKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ErrorResult("failed to reach the Akashic Library: " + err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return ErrorResult(fmt.Sprintf("knowledge store rejected (status %d): %s", resp.StatusCode, string(b)))
	}

	return NewToolResult(fmt.Sprintf("Knowledge inscribed in the Akashic Library: \"%s\" (confidence: %.2f, tags: %s)", fact, confidence, strings.Join(tags, ", ")))
}

// --- siam_recall Tool ---

// RecallTool lets an agent semantically query the Swarm's shared knowledge base
type RecallTool struct {
	MasterURL string
	APIKey    string
}

func NewRecallTool(masterURL, apiKey string) *RecallTool {
	if masterURL == "" {
		masterURL = os.Getenv("MASTER_API_URL")
		if masterURL == "" {
			masterURL = "http://master:8080"
		}
	}
	if apiKey == "" {
		apiKey = os.Getenv("MASTER_API_KEY")
	}
	return &RecallTool{MasterURL: masterURL, APIKey: apiKey}
}

func (t *RecallTool) Name() string { return "siam_recall" }

func (t *RecallTool) Description() string {
	return "Search the Swarm's shared Akashic Library for relevant knowledge. Uses semantic similarity search (not just keyword matching), so you can describe what you're looking for in natural language. Returns relevant facts with confidence scores."
}

func (t *RecallTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "What you want to know or search for. Be descriptive — semantic search understands meaning, not just keywords.",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of results to return (default: 5, max: 20)",
			},
		},
		"required": []string{"query"},
	}
}

func (t *RecallTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	query, ok := args["query"].(string)
	if !ok || strings.TrimSpace(query) == "" {
		return ErrorResult("'query' is required and must be a non-empty string")
	}

	limit := 5
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
		if limit > 20 {
			limit = 20
		}
	}

	apiURL := fmt.Sprintf("%s/api/agent/v1/knowledge/search?q=%s&limit=%d",
		t.MasterURL, url.QueryEscape(query), limit)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return ErrorResult("failed to build request: " + err.Error())
	}
	if t.APIKey != "" {
		req.Header.Set("X-API-Key", t.APIKey)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ErrorResult("failed to query the Akashic Library: " + err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return ErrorResult(fmt.Sprintf("knowledge search failed (status %d): %s", resp.StatusCode, string(b)))
	}

	var result struct {
		Results []struct {
			ID          int     `json:"id"`
			Fact        string  `json:"fact"`
			Confidence  float64 `json:"confidence"`
			SourceAgent string  `json:"source_agent"`
			Tags        []string `json:"tags"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ErrorResult("failed to parse recall response: " + err.Error())
	}

	if len(result.Results) == 0 {
		return NewToolResult(fmt.Sprintf("No knowledge found in the Akashic Library for: \"%s\"", query))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📚 Akashic Library recalled %d relevant facts for \"%s\":\n\n", len(result.Results), query))
	for i, r := range result.Results {
		sb.WriteString(fmt.Sprintf("%d. [%.0f%% confidence] %s", i+1, r.Confidence*100, r.Fact))
		if r.SourceAgent != "" {
			sb.WriteString(fmt.Sprintf(" (by %s)", r.SourceAgent))
		}
		if len(r.Tags) > 0 {
			sb.WriteString(fmt.Sprintf(" [%s]", strings.Join(r.Tags, ", ")))
		}
		sb.WriteString("\n")
	}

	return NewToolResult(sb.String())
}
