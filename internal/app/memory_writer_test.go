package app

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/solosw/solcode/internal/config"
	"github.com/solosw/solcode/internal/memory"
	"github.com/solosw/solcode/internal/tool"
)

func newMemoryWriterApp(t *testing.T) *App {
	t.Helper()
	cfg := config.Default()
	cfg.Memory.Enabled = true
	cfg.Memory.Dir = filepath.Join(t.TempDir(), "memory")
	store := memory.NewFileStore(cfg.Memory.Dir)
	return &App{
		Config:        cfg,
		MemoryStore:   store,
		MemoryManager: memory.NewManager(store, memory.DefaultGate{}, memory.StaticJudge{}),
	}
}

func TestAppWriteMemoryPersistsEntry(t *testing.T) {
	application := newMemoryWriterApp(t)

	result, err := application.WriteMemory(context.Background(), tool.MemoryWriteRequest{
		Text:      "Solcode tests live under unit_tests and internal package dirs.",
		Kind:      "fact",
		Scope:     "project",
		Tags:      []string{"testing"},
		SessionID: "s1",
	})
	if err != nil {
		t.Fatalf("WriteMemory() = %v", err)
	}
	if !result.Stored || result.Merged {
		t.Fatalf("result = %#v, want a newly stored entry", result)
	}
	if result.Tier != string(memory.TierLongTerm) {
		t.Fatalf("tier = %q, want %q for a fact", result.Tier, memory.TierLongTerm)
	}

	items, err := application.MemoryStore.List(context.Background())
	if err != nil {
		t.Fatalf("List() = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("stored items = %d, want 1", len(items))
	}
	if items[0].SourceSessionID != "s1" {
		t.Fatalf("source session = %q, want s1", items[0].SourceSessionID)
	}
	if items[0].Kind != memory.KindFact || items[0].Scope != memory.ScopeProject {
		t.Fatalf("kind/scope = %q/%q, want fact/project", items[0].Kind, items[0].Scope)
	}
}

func TestAppWriteMemoryTierFollowsKind(t *testing.T) {
	cases := []struct {
		kind string
		want memory.Tier
	}{
		{"fact", memory.TierLongTerm},
		{"preference", memory.TierLongTerm},
		{"constraint", memory.TierLongTerm},
		{"workflow", memory.TierProcedural},
		{"task", memory.TierShortTerm},
	}
	for _, tc := range cases {
		if got := memoryTierForWrite(tc.kind); got != tc.want {
			t.Fatalf("memoryTierForWrite(%q) = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

func TestAppWriteMemoryRejectsSensitiveContent(t *testing.T) {
	application := newMemoryWriterApp(t)

	result, err := application.WriteMemory(context.Background(), tool.MemoryWriteRequest{
		Text:      "The deploy API key is sk-ant-api03-secretvalue-do-not-store",
		SessionID: "s1",
	})
	if err != nil {
		t.Fatalf("WriteMemory() = %v", err)
	}
	if result.Stored {
		t.Fatalf("result = %#v, want the gate to reject secret-looking content", result)
	}

	items, err := application.MemoryStore.List(context.Background())
	if err != nil {
		t.Fatalf("List() = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("stored items = %d, want nothing persisted", len(items))
	}
}

func TestAppWriteMemoryMergesNearDuplicate(t *testing.T) {
	application := newMemoryWriterApp(t)
	req := tool.MemoryWriteRequest{
		Text:      "Build the CLI with go build ./cmd/solcode before manual checks.",
		Kind:      "workflow",
		SessionID: "s1",
	}
	if _, err := application.WriteMemory(context.Background(), req); err != nil {
		t.Fatalf("first WriteMemory() = %v", err)
	}

	req.Text = "Build the CLI with go build ./cmd/solcode before manual verification checks."
	result, err := application.WriteMemory(context.Background(), req)
	if err != nil {
		t.Fatalf("second WriteMemory() = %v", err)
	}
	if !result.Merged {
		t.Fatalf("result = %#v, want the near-duplicate to merge", result)
	}

	items, err := application.MemoryStore.List(context.Background())
	if err != nil {
		t.Fatalf("List() = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("stored items = %d, want the entries merged into 1", len(items))
	}
}

func TestAppWriteMemoryDisabledReturnsError(t *testing.T) {
	cfg := config.Default()
	cfg.Memory.Enabled = false
	application := &App{Config: cfg}
	if _, err := application.WriteMemory(context.Background(), tool.MemoryWriteRequest{Text: "x"}); err == nil {
		t.Fatal("expected an error when memory is disabled")
	}
}
