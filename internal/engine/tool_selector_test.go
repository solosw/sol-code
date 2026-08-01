package engine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/solosw/solcode/internal/tool"
)

type stubTool struct {
	tool.BaseTool
	name string
	desc string
}

func (s stubTool) Name() string        { return s.name }
func (s stubTool) Description() string { return s.desc }
func (s stubTool) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (s stubTool) IsReadOnly(json.RawMessage) bool { return true }
func (s stubTool) Invoke(context.Context, *tool.UseContext, json.RawMessage) (*tool.ContentBlock, error) {
	return tool.Result("ok"), nil
}

func sampleTools() []tool.Tool {
	return []tool.Tool{
		&stubTool{name: tool.AskUserToolName, desc: "ask the user"},
		&stubTool{name: tool.BashToolName, desc: "run shell commands"},
		&stubTool{name: tool.EditToolName, desc: "edit files by replacing text"},
		&stubTool{name: tool.ViewToolName, desc: "read files"},
		&stubTool{name: tool.WriteToolName, desc: "write files"},
		&stubTool{name: tool.ToolSearchToolName, desc: "search tools"},
		&stubTool{name: tool.GlobToolName, desc: "find files by pattern"},
		&stubTool{name: tool.GrepToolName, desc: "search file contents"},
		&stubTool{name: tool.LSToolName, desc: "list directories"},
		&stubTool{name: tool.TodoWriteToolName, desc: "manage todos"},
		&stubTool{name: tool.TaskToolName, desc: "spawn subagents"},
		&stubTool{name: tool.DiffToolName, desc: "diff files"},
		&stubTool{name: tool.PatchToolName, desc: "apply patches"},
		&stubTool{name: tool.SkillToolName, desc: "load skills"},
		&stubTool{name: "mcp__docs__query", desc: "Query library documentation and code examples from Context7"},
		&stubTool{name: "mcp__docs__resolve", desc: "Resolve a package name to a Context7 library id"},
		&stubTool{name: "mcp__office__cli", desc: "Create read and modify Office documents with officecli"},
		&stubTool{name: "mcp__godot__run", desc: "Run a Godot project scene"},
		&stubTool{name: "WebSearch", desc: "Search the web"},
		&stubTool{name: "Fetch", desc: "Fetch URL content"},
	}
}

func namesOf(tools []tool.Tool) map[string]bool {
	out := make(map[string]bool, len(tools))
	for _, t := range tools {
		out[t.Name()] = true
	}
	return out
}

func TestSelectToolsForTurnCoreOnlyByDefault(t *testing.T) {
	selected := SelectToolsForTurn(sampleTools(), nil, "", nil)
	got := namesOf(selected)
	if !got[tool.BashToolName] || !got[tool.EditToolName] || !got[tool.ToolSearchToolName] {
		t.Fatalf("core tools missing: %#v", got)
	}
	if got["mcp__docs__query"] || got["mcp__office__cli"] || got["WebSearch"] {
		t.Fatalf("dynamic tools should be omitted without query: %#v", got)
	}
	if len(selected) > 16 {
		t.Fatalf("selected too many tools: %d", len(selected))
	}
}

func TestSelectToolsForTurnLexicalMCPMatch(t *testing.T) {
	selected := SelectToolsForTurn(sampleTools(), nil, "query Context7 documentation library", nil)
	got := namesOf(selected)
	if !got["mcp__docs__query"] && !got["mcp__docs__resolve"] {
		t.Fatalf("expected docs MCP tools, got %#v", got)
	}
	if got["mcp__office__cli"] {
		t.Fatalf("office tool should not match docs query: %#v", got)
	}
}

func TestSelectToolsForTurnStickyEnabled(t *testing.T) {
	enabled := map[string]bool{"mcp__office__cli": true}
	selected := SelectToolsForTurn(sampleTools(), nil, "", enabled)
	got := namesOf(selected)
	if !got["mcp__office__cli"] {
		t.Fatalf("sticky tool missing: %#v", got)
	}
}

func TestSelectToolsForTurnAllowedWhitelist(t *testing.T) {
	allowed := []string{tool.BashToolName, tool.ViewToolName}
	selected := SelectToolsForTurn(sampleTools(), allowed, "office documents", nil)
	got := namesOf(selected)
	if len(selected) != 2 || !got[tool.BashToolName] || !got[tool.ViewToolName] {
		t.Fatalf("whitelist broken: %#v", got)
	}
}

func TestSelectToolsForTurnEmptyAllowedIsDynamic(t *testing.T) {
	// App sets AllowedTools: []string{} meaning unrestricted (legacy Filter behavior).
	selected := SelectToolsForTurn(sampleTools(), []string{}, "office documents", nil)
	got := namesOf(selected)
	if !got["mcp__office__cli"] {
		t.Fatalf("expected office match under empty allowed, got %#v", got)
	}
	if !got[tool.ToolSearchToolName] {
		t.Fatalf("core ToolSearch missing")
	}
}

func TestEnableToolsFromSearch(t *testing.T) {
	enabled := map[string]bool{}
	enableToolsFromSearch(sampleTools(), []byte(`{"query":"office documents","limit":3}`), enabled)
	if !enabled["mcp__office__cli"] {
		t.Fatalf("expected office tool enabled, got %#v", enabled)
	}
}

func TestToolSearchQueryJSON(t *testing.T) {
	if q := toolSearchQuery([]byte(`{"query":"godot scene","limit":2}`)); q != "godot scene" {
		t.Fatalf("query = %q", q)
	}
	if q := toolSearchQuery([]byte(`not-json`)); q != "" {
		t.Fatalf("bad input query = %q", q)
	}
}

func TestSelectionQueryJoin(t *testing.T) {
	if got := selectionQuery(" edit ", " docs "); !strings.Contains(got, "edit") || !strings.Contains(got, "docs") {
		t.Fatalf("got %q", got)
	}
}
