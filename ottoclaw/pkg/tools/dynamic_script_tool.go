package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// DynamicScriptTool is a wrapper that turns a raw script (Python, Bash, JS) into a usable tool
type DynamicScriptTool struct {
	ToolName        string
	ToolDescription string
	Language        string
	Code            string
	Schema          map[string]any
}

func NewDynamicScriptTool(name, description, language, code, jsonSchema string) *DynamicScriptTool {
	var schema map[string]any
	if jsonSchema != "" {
		if err := json.Unmarshal([]byte(jsonSchema), &schema); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️ Failed to parse dynamic tool schema for %s: %v\n", name, err)
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
	} else {
		schema = map[string]any{"type": "object", "properties": map[string]any{}}
	}

	return &DynamicScriptTool{
		ToolName:        name,
		ToolDescription: description,
		Language:        strings.ToLower(language),
		Code:            code,
		Schema:          schema,
	}
}

func (t *DynamicScriptTool) Name() string {
	return t.ToolName
}

func (t *DynamicScriptTool) Description() string {
	return t.ToolDescription
}

func (t *DynamicScriptTool) Parameters() map[string]any {
	return t.Schema
}

func (t *DynamicScriptTool) Execute(ctx context.Context, args map[string]interface{}) *ToolResult {
	// 1. Serialize arguments
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return ErrorResult("failed to serialize arguments: " + err.Error())
	}

	// 2. Prepare workspace temp file
	tmpDir := os.TempDir()
	fileName := fmt.Sprintf("forge_tool_%s", uuid.NewString())

	var ext string
	var runCmd string
	var runArgs []string

	switch t.Language {
	case "python":
		ext = ".py"
		runCmd = "python3"
	case "javascript", "js", "node":
		ext = ".js"
		runCmd = "node"
	case "bash", "shell", "sh":
		ext = ".sh"
		runCmd = "bash"
	default:
		return ErrorResult("unsupported language: " + t.Language)
	}

	scriptPath := filepath.Join(tmpDir, fileName+ext)

	// Inject the code. For some languages, we might want to also write the arguments alongside it or pass them via ENV/CLI args.
	// For simplicity, we just pass the raw json string as the first CLI argument to the script.
	if err := os.WriteFile(scriptPath, []byte(t.Code), 0755); err != nil {
		return ErrorResult("failed to write script: " + err.Error())
	}
	defer os.Remove(scriptPath)

	runArgs = append(runArgs, scriptPath, string(argsJSON))

	// 3. Execute script
	cmd := exec.CommandContext(ctx, runCmd, runArgs...)
	
	// Pass JSON args as ENV variable too, for easier parsing in bash
	cmd.Env = append(os.Environ(), fmt.Sprintf("TOOL_ARGS=%s", string(argsJSON)))

	out, err := cmd.CombinedOutput()
	
	outputStr := string(out)

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return ErrorResult("script execution timed out")
		}
		return ErrorResult(fmt.Sprintf("script failed with exit code: %v\nOutput: %s", err, outputStr))
	}

	return NewToolResult(fmt.Sprintf("Executed Dynamic Tool %s successfully\nOutput: %s", t.ToolName, outputStr))
}
