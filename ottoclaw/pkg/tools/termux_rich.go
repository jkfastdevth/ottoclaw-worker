package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// TermuxRichTool renders beautiful terminal output using Python's rich library.
// Designed for Termux (Android) and any Linux node with Python3+rich installed.
type TermuxRichTool struct{}

func NewTermuxRichTool() *TermuxRichTool {
	return &TermuxRichTool{}
}

func (t *TermuxRichTool) Name() string {
	return "termux_rich"
}

func (t *TermuxRichTool) Description() string {
	return "Render beautiful formatted terminal output using Python's rich library. " +
		"Supports: panel (bordered box), table (grid), markdown, tree (hierarchy), " +
		"rule (divider), status (indicator), text (styled). " +
		"Requires: python3 + rich (install with: pip install rich). " +
		"Use this to display structured reports, summaries, or data beautifully in the terminal."
}

func (t *TermuxRichTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"format": map[string]any{
				"type": "string",
				"enum": []string{"panel", "table", "markdown", "tree", "rule", "status", "text"},
				"description": "Output format: " +
					"panel=bordered box, table=data grid, markdown=rich markdown rendering, " +
					"tree=hierarchy display, rule=horizontal divider, status=status indicator, text=styled text",
			},
			"content": map[string]any{
				"type": "string",
				"description": "Content to render. " +
					"For 'table': JSON array of row arrays e.g. [[\"Name\",\"Value\"],[\"CPU\",\"20%\"]]. " +
					"For 'tree': JSON object e.g. {\"Root\":{\"Branch1\":\"leaf\",\"Branch2\":\"leaf\"}}. " +
					"For others: plain text or markdown string.",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "Optional title for panel, table, or rule",
			},
			"columns": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Column headers for table format (e.g. [\"Name\", \"Status\", \"Value\"])",
			},
			"style": map[string]any{
				"type":        "string",
				"description": "Rich style string e.g. 'bold green', 'cyan', 'red on white', 'bold blue'. Default: 'default'",
			},
			"border_style": map[string]any{
				"type":        "string",
				"enum":        []string{"rounded", "heavy", "double", "ascii", "simple", "minimal"},
				"description": "Border style for panel/table. Default: 'rounded'",
			},
		},
		"required": []string{"format", "content"},
	}
}

func (t *TermuxRichTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	// Validate required fields
	format, _ := args["format"].(string)
	if format == "" {
		format = "text"
	}
	content, ok := args["content"].(string)
	if !ok || content == "" {
		return ErrorResult("content is required")
	}

	// Build safe args payload for Python (pass via stdin to avoid escaping issues)
	payload := map[string]any{
		"format":       format,
		"content":      content,
		"title":        args["title"],
		"style":        args["style"],
		"border_style": args["border_style"],
		"columns":      args["columns"],
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to marshal args: %v", err))
	}

	script := richPythonScript()
	cmd := exec.CommandContext(ctx, "python3", "-c", script)
	cmd.Stdin = bytes.NewReader(payloadJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if strings.Contains(errMsg, "No module named 'rich'") {
			return ErrorResult("rich library not installed. Install with: pip install rich")
		}
		if strings.Contains(errMsg, "python3: not found") || strings.Contains(errMsg, "No such file") {
			return ErrorResult("python3 not found. Install with: pkg install python (Termux) or apt install python3")
		}
		return ErrorResult(fmt.Sprintf("rich render failed: %v\n%s", err, errMsg))
	}

	output := stdout.String()
	if output == "" {
		output = "(no output)"
	}
	// Strip ANSI escape codes for LLM context, keep full output for user
	return &ToolResult{
		ForLLM:  stripANSI(output),
		ForUser: output,
	}
}

// richPythonScript returns the Python script that reads JSON from stdin and renders rich output.
func richPythonScript() string {
	return `
import sys, json
from rich.console import Console
from rich.panel import Panel
from rich.table import Table
from rich.markdown import Markdown
from rich.tree import Tree
from rich.rule import Rule
from rich.text import Text
from rich import box as rbox

args = json.load(sys.stdin)
fmt    = args.get("format", "text")
content = args.get("content", "")
title   = args.get("title") or None
style   = args.get("style") or "default"
border  = args.get("border_style") or "rounded"
columns = args.get("columns") or []

BOX_MAP = {
    "rounded": rbox.ROUNDED,
    "heavy":   rbox.HEAVY,
    "double":  rbox.DOUBLE,
    "ascii":   rbox.ASCII,
    "simple":  rbox.SIMPLE,
    "minimal": rbox.MINIMAL,
}
box_style = BOX_MAP.get(border, rbox.ROUNDED)

c = Console()

if fmt == "panel":
    c.print(Panel(content, title=title, border_style=style if style != "default" else "blue", box=box_style))

elif fmt == "table":
    t = Table(title=title, box=box_style, show_header=bool(columns))
    for col in columns:
        t.add_column(col, style="cyan", header_style="bold cyan")
    try:
        rows = json.loads(content)
        if not isinstance(rows, list):
            rows = [[str(rows)]]
        for row in rows:
            if isinstance(row, list):
                t.add_row(*[str(v) for v in row])
            else:
                t.add_row(str(row))
    except json.JSONDecodeError:
        # Treat as plain text rows (one per line)
        for line in content.splitlines():
            if line.strip():
                t.add_row(line.strip())
    c.print(t)

elif fmt == "markdown":
    c.print(Markdown(content))

elif fmt == "tree":
    root_label = f"[bold]{title}[/bold]" if title else "[bold]Tree[/bold]"
    root = Tree(root_label)
    def build(node, branch):
        if isinstance(node, dict):
            for k, v in node.items():
                child = branch.add(f"[bold cyan]{k}[/bold cyan]")
                build(v, child)
        elif isinstance(node, list):
            for item in node:
                build(item, branch)
        else:
            branch.add(str(node))
    try:
        data = json.loads(content)
        build(data, root)
    except json.JSONDecodeError:
        root.add(content)
    c.print(root)

elif fmt == "rule":
    rule_title = title or content
    c.print(Rule(f"[bold]{rule_title}[/bold]", style=style if style != "default" else "blue"))

elif fmt == "status":
    t = Text()
    t.append("● ", style="bold green")
    t.append(content, style=style if style != "default" else "default")
    if title:
        t.append(f"  [{title}]", style="dim")
    c.print(t)

else:  # text
    t = Text(content, style=style if style != "default" else "default")
    if title:
        c.print(f"[bold]{title}[/bold]")
    c.print(t)
`
}

// stripANSI removes ANSI escape sequences from a string (for LLM context).
func stripANSI(s string) string {
	result := strings.Builder{}
	inEscape := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if c == 'm' || c == 'K' || c == 'J' || c == 'H' || c == 'A' || c == 'B' || c == 'C' || c == 'D' {
				inEscape = false
			}
			continue
		}
		result.WriteByte(c)
	}
	return result.String()
}
