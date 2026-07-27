package unit_tests

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/solosw/solcode/internal/tool"
)

type fakeMemoryReader struct {
	got    tool.MemoryReadRequest
	calls  int
	result tool.MemoryReadResult
}

func (f *fakeMemoryReader) ReadMemory(_ context.Context, req tool.MemoryReadRequest) (tool.MemoryReadResult, error) {
	f.calls++
	f.got = req
	return f.result, nil
}

func invokeReadMemory(t *testing.T, reader tool.MemoryReader, params map[string]any) *tool.ContentBlock {
	t.Helper()
	rm := tool.NewReadMemoryTool(reader)
	input, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	uctx := &tool.UseContext{SessionID: "s1", WorkDir: t.TempDir()}
	result, err := rm.Invoke(context.Background(), uctx, input)
	if err != nil {
		t.Fatalf("Invoke() = %v", err)
	}
	return result
}

func TestReadMemoryToolListsEntries(t *testing.T) {
	reader := &fakeMemoryReader{result: tool.MemoryReadResult{
		CrossSessionAllowed: true,
		Entries: []tool.MemoryEntry{
			{Text: "Build with go build ./cmd/solcode.", Tier: "M5", Kind: "workflow", Scope: "project", Tags: []string{"build"}},
			{Text: "User prefers concise replies.", Tier: "M4", Kind: "preference", Scope: "global", OtherSession: true},
		},
	}}

	result := invokeReadMemory(t, reader, map[string]any{"query": "  build commands  ", "limit": 5})

	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Text)
	}
	if reader.calls != 1 {
		t.Fatalf("reader calls = %d, want 1", reader.calls)
	}
	if reader.got.Query != "build commands" {
		t.Fatalf("query = %q, want trimmed query", reader.got.Query)
	}
	if reader.got.Limit != 5 {
		t.Fatalf("limit = %d, want 5", reader.got.Limit)
	}
	if reader.got.SessionID != "s1" {
		t.Fatalf("session id = %q, want s1", reader.got.SessionID)
	}
	for _, want := range []string{"2 stored memories", "workflow/project", "other-session", "tags: build"} {
		if !strings.Contains(result.Text, want) {
			t.Fatalf("result = %q, want mention of %q", result.Text, want)
		}
	}
	if strings.Contains(result.Text, "opted out") {
		t.Fatalf("result = %q, should not warn when cross-session is allowed", result.Text)
	}
}

func TestReadMemoryToolDefaultsAndClampsLimit(t *testing.T) {
	reader := &fakeMemoryReader{result: tool.MemoryReadResult{CrossSessionAllowed: true}}
	invokeReadMemory(t, reader, map[string]any{})
	if reader.got.Limit != 8 {
		t.Fatalf("default limit = %d, want 8", reader.got.Limit)
	}
	if reader.got.Kind != "" || reader.got.Scope != "" {
		t.Fatalf("filters = %q/%q, want empty by default", reader.got.Kind, reader.got.Scope)
	}

	invokeReadMemory(t, reader, map[string]any{"limit": 500})
	if reader.got.Limit != 25 {
		t.Fatalf("clamped limit = %d, want 25", reader.got.Limit)
	}
}

func TestReadMemoryToolNormalizesFilters(t *testing.T) {
	reader := &fakeMemoryReader{result: tool.MemoryReadResult{CrossSessionAllowed: true}}
	invokeReadMemory(t, reader, map[string]any{"kind": "Workflow", "scope": "GLOBAL"})
	if reader.got.Kind != "workflow" || reader.got.Scope != "global" {
		t.Fatalf("filters = %q/%q, want normalized lowercase", reader.got.Kind, reader.got.Scope)
	}
}

func TestReadMemoryToolRejectsInvalidFilters(t *testing.T) {
	cases := []struct {
		name   string
		params map[string]any
		want   string
	}{
		{"bad kind", map[string]any{"kind": "trivia"}, "invalid kind"},
		{"bad scope", map[string]any{"scope": "universe"}, "invalid scope"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader := &fakeMemoryReader{}
			result := invokeReadMemory(t, reader, tc.params)
			if !result.IsError || !strings.Contains(result.Text, tc.want) {
				t.Fatalf("result = %q (isError=%v), want %q", result.Text, result.IsError, tc.want)
			}
			if reader.calls != 0 {
				t.Fatalf("reader should not be called for invalid filters, got %d calls", reader.calls)
			}
		})
	}
}

func TestReadMemoryToolReportsEmptyAndOptOut(t *testing.T) {
	reader := &fakeMemoryReader{result: tool.MemoryReadResult{CrossSessionAllowed: false}}
	result := invokeReadMemory(t, reader, map[string]any{"query": "deploy steps"})
	if result.IsError {
		t.Fatalf("empty result should not be an error: %s", result.Text)
	}
	if !strings.Contains(result.Text, "No stored memory matched") {
		t.Fatalf("result = %q, want empty-result message", result.Text)
	}
	if !strings.Contains(result.Text, "opted out of cross-session memory") {
		t.Fatalf("result = %q, want cross-session opt-out note", result.Text)
	}
}

func TestReadMemoryToolWithoutReader(t *testing.T) {
	result := invokeReadMemory(t, nil, map[string]any{"query": "anything"})
	if !result.IsError || !strings.Contains(result.Text, "not enabled") {
		t.Fatalf("result = %q (isError=%v), want disabled-memory error", result.Text, result.IsError)
	}
}

func TestReadMemoryToolSafetyFlags(t *testing.T) {
	rm := tool.NewReadMemoryTool(&fakeMemoryReader{})
	if rm.Name() != "ReadMemory" {
		t.Fatalf("name = %q, want ReadMemory", rm.Name())
	}
	if !rm.IsReadOnly(nil) {
		t.Fatal("ReadMemory only reads; it must be read-only so plan mode allows it")
	}
	if rm.IsDestructive(nil) {
		t.Fatal("ReadMemory must not be destructive")
	}
	if !rm.IsConcurrencySafe(nil) {
		t.Fatal("ReadMemory should be concurrency safe")
	}
}
