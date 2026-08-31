package acp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/solosw/solcode/internal/tool"
)

func TestToolCallDiffsEditAndWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(path, []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	editInput, _ := json.Marshal(tool.EditParams{
		Path:  path,
		OldString: "world",
		NewString: "solcode",
	})
	content, locations := toolCallDiffs(tool.EditToolName, editInput, dir)
	if len(content) != 1 || content[0].Type != "diff" {
		t.Fatalf("edit diffs = %+v", content)
	}
	if content[0].OldText == nil || *content[0].OldText != "hello world\n" {
		t.Fatalf("edit oldText = %v", content[0].OldText)
	}
	if content[0].NewText != "hello solcode\n" {
		t.Fatalf("edit newText = %q", content[0].NewText)
	}
	if len(locations) != 1 || locations[0].Path != path {
		t.Fatalf("edit locations = %+v", locations)
	}

	// Ensure ACP JSON uses null oldText for new files.
	newPath := filepath.Join(dir, "created.txt")
	writeInput, _ := json.Marshal(tool.WriteParams{
		Path: newPath,
		Content:  "brand new\n",
	})
	content, locations = toolCallDiffs(tool.WriteToolName, writeInput, dir)
	if len(content) != 1 || content[0].Type != "diff" || content[0].OldText != nil {
		t.Fatalf("write new-file diffs = %+v", content)
	}
	if content[0].NewText != "brand new\n" {
		t.Fatalf("write newText = %q", content[0].NewText)
	}
	raw, err := json.Marshal(content[0])
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["type"] != "diff" || payload["path"] != newPath {
		t.Fatalf("marshaled diff = %s", raw)
	}
	if payload["oldText"] != nil {
		t.Fatalf("expected null oldText, got %#v in %s", payload["oldText"], raw)
	}
	if len(locations) != 1 || locations[0].Path != newPath {
		t.Fatalf("write locations = %+v", locations)
	}
}

func TestToolCallDiffsMultiWrite(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(existing, []byte("old-a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	created := filepath.Join(dir, "b.txt")
	input, _ := json.Marshal(tool.MultiWriteParams{
		Files: []tool.WriteParams{
			{Path: existing, Content: "new-a\n"},
			{Path: created, Content: "new-b\n"},
		},
	})
	content, locations := toolCallDiffs(tool.MultiWriteToolName, input, dir)
	if len(content) != 2 || len(locations) != 2 {
		t.Fatalf("multiwrite content=%d locations=%d", len(content), len(locations))
	}
	if content[0].OldText == nil || *content[0].OldText != "old-a\n" || content[0].NewText != "new-a\n" {
		t.Fatalf("first diff = %+v", content[0])
	}
	if content[1].OldText != nil || content[1].NewText != "new-b\n" {
		t.Fatalf("second diff = %+v", content[1])
	}
}

func TestSessionUpdateMarshalsToolDiffContent(t *testing.T) {
	old := "before\n"
	update := SessionUpdate{
		SessionUpdate: "tool_call",
		ToolCallID:    "tool-1",
		Title:         "Edit",
		Kind:          "edit",
		Status:        ToolCallInProgress,
		ToolContent: []ToolCallContent{{
			Type:    "diff",
			Path:    "/tmp/x.go",
			OldText: &old,
			NewText: "after\n",
		}},
		Locations: []ToolCallLocation{{Path: "/tmp/x.go"}},
	}
	raw, err := json.Marshal(update)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	content, ok := payload["content"].([]any)
	if !ok || len(content) != 1 {
		t.Fatalf("content = %#v", payload["content"])
	}
	item := content[0].(map[string]any)
	if item["type"] != "diff" || item["path"] != "/tmp/x.go" || item["newText"] != "after\n" {
		t.Fatalf("diff item = %#v", item)
	}
	if item["oldText"] != "before\n" {
		t.Fatalf("oldText = %#v", item["oldText"])
	}
	locs, ok := payload["locations"].([]any)
	if !ok || len(locs) != 1 {
		t.Fatalf("locations = %#v", payload["locations"])
	}
}
