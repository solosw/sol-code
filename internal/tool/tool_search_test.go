package tool_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/solosw/solcode/internal/skill"
	"github.com/solosw/solcode/internal/tool"
)

type stubTool struct {
	tool.BaseTool
	name string
	desc string
}

func (s stubTool) Name() string                    { return s.name }
func (s stubTool) Description() string             { return s.desc }
func (s stubTool) InputSchema() map[string]any     { return map[string]any{"type": "object"} }
func (s stubTool) IsReadOnly(json.RawMessage) bool { return true }
func (s stubTool) Invoke(context.Context, *tool.UseContext, json.RawMessage) (*tool.ContentBlock, error) {
	return tool.Result("ok"), nil
}

func TestToolSearchFindsLiveMCPTools(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(
		&stubTool{name: "Bash", desc: "run shell commands"},
		&stubTool{name: "mcp__docs__query", desc: "Query library documentation and code examples"},
		&stubTool{name: "mcp__docs__resolve", desc: "Resolve a library id from a package name"},
		tool.NewToolSearchTool(reg, nil),
	)

	search := tool.NewToolSearchTool(reg, nil)
	out, err := search.Invoke(context.Background(), nil, json.RawMessage(`{"query":"documentation library","limit":3}`))
	if err != nil {
		t.Fatal(err)
	}
	if out == nil || out.IsError {
		t.Fatalf("unexpected error result: %#v", out)
	}
	text := out.Text
	if !strings.Contains(text, "mcp__docs__query") {
		t.Fatalf("expected docs query tool, got %q", text)
	}
	if strings.Contains(text, "Bash") {
		t.Fatalf("did not expect Bash for docs query: %q", text)
	}
}

func TestSearchCapabilitiesReturnsToolNames(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(
		&stubTool{name: "mcp__office__run", desc: "Create and edit Office documents"},
		&stubTool{name: "mcp__godot__run", desc: "Run a Godot project"},
	)
	skills := skill.NewRegistry()
	// skills registry may be empty; search still works for tools
	matches, err := tool.SearchCapabilities(reg, skills, json.RawMessage(`{"query":"office documents","limit":5}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("expected matches")
	}
	if matches[0].Name != "mcp__office__run" {
		t.Fatalf("top match = %s", matches[0].Name)
	}
}

func TestCapabilityScoreLexical(t *testing.T) {
	score := tool.CapabilityScore("edit source files", "Edit", "Edits files by replacing text")
	if score <= 0 {
		t.Fatalf("expected positive score, got %d", score)
	}
	zero := tool.CapabilityScore("zzzqqq", "Edit", "Edits files by replacing text")
	if zero != 0 {
		t.Fatalf("expected zero score, got %d", zero)
	}
}
