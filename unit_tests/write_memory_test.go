package unit_tests

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/solosw/solcode/internal/tool"
)

type fakeMemoryWriter struct {
	got    tool.MemoryWriteRequest
	calls  int
	result tool.MemoryWriteResult
	err    error
}

func (f *fakeMemoryWriter) WriteMemory(_ context.Context, req tool.MemoryWriteRequest) (tool.MemoryWriteResult, error) {
	f.calls++
	f.got = req
	return f.result, f.err
}

func invokeWriteMemory(t *testing.T, writer tool.MemoryWriter, params map[string]any) *tool.ContentBlock {
	t.Helper()
	wm := tool.NewWriteMemoryTool(writer)
	input, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	uctx := &tool.UseContext{SessionID: "s1", WorkDir: t.TempDir()}
	result, err := wm.Invoke(context.Background(), uctx, input)
	if err != nil {
		t.Fatalf("Invoke() = %v", err)
	}
	return result
}

func TestWriteMemoryToolStoresEntry(t *testing.T) {
	writer := &fakeMemoryWriter{result: tool.MemoryWriteResult{
		ID:     "mem-1",
		Text:   "User prefers table-driven tests.",
		Tier:   "M4",
		Kind:   "preference",
		Scope:  "project",
		Stored: true,
	}}

	result := invokeWriteMemory(t, writer, map[string]any{
		"memory": "  User prefers table-driven tests.  ",
		"kind":   "Preference",
		"scope":  "PROJECT",
		"tags":   []string{"Testing", "testing", " "},
	})

	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Text)
	}
	if writer.calls != 1 {
		t.Fatalf("writer calls = %d, want 1", writer.calls)
	}
	if writer.got.Text != "User prefers table-driven tests." {
		t.Fatalf("text = %q, want trimmed text", writer.got.Text)
	}
	if writer.got.Kind != "preference" || writer.got.Scope != "project" {
		t.Fatalf("kind/scope = %q/%q, want normalized lowercase", writer.got.Kind, writer.got.Scope)
	}
	if len(writer.got.Tags) != 1 || writer.got.Tags[0] != "testing" {
		t.Fatalf("tags = %v, want deduped [testing]", writer.got.Tags)
	}
	if writer.got.SessionID != "s1" {
		t.Fatalf("session id = %q, want s1", writer.got.SessionID)
	}
	if !strings.Contains(result.Text, "Memory stored") || !strings.Contains(result.Text, "M4") {
		t.Fatalf("result text = %q, want stored confirmation with tier", result.Text)
	}
}

func TestWriteMemoryToolDefaultsKindAndScope(t *testing.T) {
	writer := &fakeMemoryWriter{result: tool.MemoryWriteResult{Stored: true, Kind: "fact", Scope: "project", Tier: "M4"}}
	result := invokeWriteMemory(t, writer, map[string]any{"memory": "Build with go build ./cmd/solcode."})
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Text)
	}
	if writer.got.Kind != "fact" || writer.got.Scope != "project" {
		t.Fatalf("defaults = %q/%q, want fact/project", writer.got.Kind, writer.got.Scope)
	}
}

func TestWriteMemoryToolRejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name   string
		params map[string]any
		want   string
	}{
		{"empty memory", map[string]any{"memory": "   "}, "memory is required"},
		{"bad kind", map[string]any{"memory": "x", "kind": "trivia"}, "invalid kind"},
		{"bad scope", map[string]any{"memory": "x", "scope": "universe"}, "invalid scope"},
		{"too long", map[string]any{"memory": strings.Repeat("a", 1300)}, "too long"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writer := &fakeMemoryWriter{}
			result := invokeWriteMemory(t, writer, tc.params)
			if !result.IsError {
				t.Fatalf("expected error result, got %q", result.Text)
			}
			if !strings.Contains(result.Text, tc.want) {
				t.Fatalf("result = %q, want mention of %q", result.Text, tc.want)
			}
			if writer.calls != 0 {
				t.Fatalf("writer should not be called for invalid input, got %d calls", writer.calls)
			}
		})
	}
}

func TestWriteMemoryToolReportsRejectionAndMerge(t *testing.T) {
	rejected := &fakeMemoryWriter{result: tool.MemoryWriteResult{Stored: false, Reason: "sensitive"}}
	result := invokeWriteMemory(t, rejected, map[string]any{"memory": "token is abc"})
	if result.IsError {
		t.Fatalf("gate rejection should be a normal result: %s", result.Text)
	}
	if !strings.Contains(result.Text, "not stored") || !strings.Contains(result.Text, "sensitive") {
		t.Fatalf("result = %q, want rejection reason", result.Text)
	}

	merged := &fakeMemoryWriter{result: tool.MemoryWriteResult{
		Stored: true, Merged: true, Kind: "fact", Scope: "project", Text: "merged text",
	}}
	result = invokeWriteMemory(t, merged, map[string]any{"memory": "same-ish fact"})
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Text)
	}
	if !strings.Contains(result.Text, "Merged into an existing memory") {
		t.Fatalf("result = %q, want merge confirmation", result.Text)
	}
}

func TestWriteMemoryToolWithoutWriter(t *testing.T) {
	result := invokeWriteMemory(t, nil, map[string]any{"memory": "anything"})
	if !result.IsError || !strings.Contains(result.Text, "not enabled") {
		t.Fatalf("result = %q (isError=%v), want disabled-memory error", result.Text, result.IsError)
	}
}

func TestWriteMemoryToolSafetyFlags(t *testing.T) {
	wm := tool.NewWriteMemoryTool(&fakeMemoryWriter{})
	if wm.Name() != "WriteMemory" {
		t.Fatalf("name = %q, want WriteMemory", wm.Name())
	}
	if wm.IsReadOnly(nil) {
		t.Fatal("WriteMemory mutates the memory store; it is not read-only")
	}
	if wm.IsDestructive(nil) {
		t.Fatal("WriteMemory should not require destructive-action confirmation")
	}
	if wm.IsConcurrencySafe(nil) {
		t.Fatal("WriteMemory writes a shared store and must not run concurrently")
	}
}
