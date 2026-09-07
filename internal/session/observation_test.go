package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sdk "github.com/anthropics/anthropic-sdk-go"
)

type recordingSummaryWriter struct {
	called int
}

func (w *recordingSummaryWriter) Summarize(context.Context, string, string) (string, error) {
	w.called++
	return "llm summary", nil
}

func TestMaskObservationsStoresAndRetrievesOriginal(t *testing.T) {
	dir := t.TempDir()
	store := NewFileObservationStore(dir)
	oldOutput := strings.Repeat("old fetch body ", 80)
	recentOutput := strings.Repeat("recent fetch body ", 80)
	messages := []sdk.MessageParam{
		sdk.NewUserMessage(sdk.NewTextBlock("old user")),
		sdk.NewAssistantMessage(sdk.NewToolUseBlock("toolu_old", map[string]any{"url": "https://old.example"}, "Fetch")),
		sdk.NewUserMessage(sdk.NewToolResultBlock("toolu_old", oldOutput, false)),
		sdk.NewAssistantMessage(sdk.NewTextBlock("old reasoning")),
		sdk.NewUserMessage(sdk.NewTextBlock("recent user")),
		sdk.NewAssistantMessage(sdk.NewToolUseBlock("toolu_new", map[string]any{"url": "https://new.example"}, "Fetch")),
		sdk.NewUserMessage(sdk.NewToolResultBlock("toolu_new", recentOutput, false)),
	}

	result, err := MaskObservations(messages, store, ObservationMaskOptions{RecentUnmaskedTurns: 1, MinChars: 50})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.Masked != 1 {
		t.Fatalf("mask result = %+v", result)
	}

	oldText := observationContent(result.Messages[2].Content[0].OfToolResult)
	if !strings.Contains(oldText, ObservationMaskMarker) {
		t.Fatalf("old observation not masked: %q", oldText)
	}
	if strings.HasPrefix(oldText, "old fetch body") {
		t.Fatalf("old observation still contains full payload: %q", oldText)
	}
	recentText := observationContent(result.Messages[6].Content[0].OfToolResult)
	if !strings.Contains(recentText, "recent fetch body") {
		t.Fatalf("recent observation should stay: %q", recentText)
	}

	if strings.Contains(oldText, "preview=") || strings.Contains(oldText, "path=") || strings.Contains(oldText, "\n") {
		t.Fatalf("placeholder should be a short id marker, got %q", oldText)
	}
	id, _ := ParseObservationRef(oldText)
	if id == "" {
		t.Fatalf("expected observation_id in %q", oldText)
	}
	loaded, err := store.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != strings.TrimSpace(oldOutput) {
		t.Fatalf("loaded observation mismatch")
	}
	disk, err := os.ReadFile(filepath.Join(dir, id+".txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(disk) != loaded {
		t.Fatalf("path content mismatch: %q vs %q", disk, loaded)
	}
}

func TestMaskObservationsKeepsNewestTwoTurnsByDefault(t *testing.T) {
	oldOutput := strings.Repeat("old fetch body ", 80)
	recentOutput := strings.Repeat("recent fetch body ", 80)
	messages := []sdk.MessageParam{
		sdk.NewUserMessage(sdk.NewTextBlock("oldest user")),
		sdk.NewAssistantMessage(sdk.NewToolUseBlock("toolu_old", map[string]any{"url": "https://old.example"}, "Fetch")),
		sdk.NewUserMessage(sdk.NewToolResultBlock("toolu_old", oldOutput, false)),
		sdk.NewAssistantMessage(sdk.NewTextBlock("old reasoning")),
		sdk.NewUserMessage(sdk.NewTextBlock("middle user")),
		sdk.NewAssistantMessage(sdk.NewTextBlock("middle assistant")),
		sdk.NewUserMessage(sdk.NewTextBlock("recent user")),
		sdk.NewAssistantMessage(sdk.NewToolUseBlock("toolu_new", map[string]any{"url": "https://new.example"}, "Fetch")),
		sdk.NewUserMessage(sdk.NewToolResultBlock("toolu_new", recentOutput, false)),
	}

	result, err := MaskObservations(messages, nil, ObservationMaskOptions{MinChars: 50})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.Masked != 1 {
		t.Fatalf("mask result = %+v", result)
	}
	oldText := observationContent(result.Messages[2].Content[0].OfToolResult)
	if !strings.Contains(oldText, ObservationMaskMarker) {
		t.Fatalf("oldest observation not masked: %q", oldText)
	}
	recentText := observationContent(result.Messages[8].Content[0].OfToolResult)
	if !strings.Contains(recentText, "recent fetch body") {
		t.Fatalf("recent observation should stay: %q", recentText)
	}
}

func TestMaskObservationsMasksEditAndWriteResults(t *testing.T) {
	editOutput := strings.Repeat("updated auth.go contents ", 40)
	writeOutput := strings.Repeat("wrote config.yaml contents ", 40)
	recentOutput := strings.Repeat("recent fetch body ", 40)
	messages := []sdk.MessageParam{
		sdk.NewUserMessage(sdk.NewTextBlock("oldest user")),
		sdk.NewAssistantMessage(sdk.NewToolUseBlock("toolu_edit", map[string]any{"path": "auth.go"}, "Edit")),
		sdk.NewUserMessage(sdk.NewToolResultBlock("toolu_edit", editOutput, false)),
		sdk.NewAssistantMessage(sdk.NewTextBlock("edited auth.go")),
		sdk.NewUserMessage(sdk.NewTextBlock("middle user")),
		sdk.NewAssistantMessage(sdk.NewToolUseBlock("toolu_write", map[string]any{"path": "config.yaml"}, "Write")),
		sdk.NewUserMessage(sdk.NewToolResultBlock("toolu_write", writeOutput, false)),
		sdk.NewAssistantMessage(sdk.NewTextBlock("wrote config.yaml")),
		sdk.NewUserMessage(sdk.NewTextBlock("recent user")),
		sdk.NewAssistantMessage(sdk.NewToolUseBlock("toolu_fetch", map[string]any{"url": "https://new.example"}, "Fetch")),
		sdk.NewUserMessage(sdk.NewToolResultBlock("toolu_fetch", recentOutput, false)),
	}

	result, err := MaskObservations(messages, nil, ObservationMaskOptions{RecentUnmaskedTurns: 1, MinChars: 50})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.Masked != 2 {
		t.Fatalf("mask result = %+v", result)
	}

	editText := observationContent(result.Messages[2].Content[0].OfToolResult)
	if !strings.Contains(editText, ObservationMaskMarker) || !strings.Contains(editText, "tool=Edit") {
		t.Fatalf("Edit observation not masked: %q", editText)
	}
	writeText := observationContent(result.Messages[6].Content[0].OfToolResult)
	if !strings.Contains(writeText, ObservationMaskMarker) || !strings.Contains(writeText, "tool=Write") {
		t.Fatalf("Write observation not masked: %q", writeText)
	}
	if result.Messages[1].Content[0].OfToolUse == nil || result.Messages[1].Content[0].OfToolUse.Name != "Edit" {
		t.Fatal("expected Edit tool-use action to remain")
	}
	if result.Messages[5].Content[0].OfToolUse == nil || result.Messages[5].Content[0].OfToolUse.Name != "Write" {
		t.Fatal("expected Write tool-use action to remain")
	}
	recentText := observationContent(result.Messages[10].Content[0].OfToolResult)
	if !strings.Contains(recentText, "recent fetch body") {
		t.Fatalf("recent observation should stay: %q", recentText)
	}
}

func TestCompactMasksObservationsWithoutLLMSummaryWhenUnderGate(t *testing.T) {
	writer := &recordingSummaryWriter{}
	oldOutput := strings.Repeat("fetch output ", 200)
	messages := []sdk.MessageParam{
		sdk.NewUserMessage(sdk.NewTextBlock("old user")),
		sdk.NewAssistantMessage(sdk.NewToolUseBlock("toolu_old", map[string]any{"url": "https://example.com"}, "Fetch")),
		sdk.NewUserMessage(sdk.NewToolResultBlock("toolu_old", oldOutput, false)),
		sdk.NewAssistantMessage(sdk.NewTextBlock("old reasoning")),
		sdk.NewUserMessage(sdk.NewTextBlock("recent user")),
		sdk.NewAssistantMessage(sdk.NewTextBlock("recent assistant")),
	}
	store := NewFileObservationStore(t.TempDir())
	result, err := Compact(context.Background(), "previous", messages, writer, CompactOptions{
		MaxRecentTurns:         20,
		SummaryThresholdTokens: 1,
		EstimatedTokens:        1000,
		ObservationStore:       store,
		ObservationKeepTurns:   1,
		ObservationMinChars:    50,
		SummaryIfAboveTokens:   1_000_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if writer.called != 0 {
		t.Fatalf("LLM summary should be skipped after successful masking, called=%d", writer.called)
	}
	if !result.Changed {
		t.Fatal("expected masking to change the session")
	}
	if !strings.Contains(Transcript(result.Messages), ObservationMaskMarker) {
		t.Fatalf("expected masked observation, got %q", Transcript(result.Messages))
	}
	if !strings.Contains(Transcript(result.Messages), "old reasoning") {
		t.Fatal("expected reasoning to remain")
	}
}

func TestCompactUsesLLMSummaryWhenMaskingLeavesHistoryAboveGate(t *testing.T) {
	writer := &recordingSummaryWriter{}
	messages := []sdk.MessageParam{}
	for i := 0; i < 8; i++ {
		messages = append(messages,
			sdk.NewUserMessage(sdk.NewTextBlock("user turn with some text")),
			sdk.NewAssistantMessage(sdk.NewTextBlock("assistant turn with some text")),
		)
	}
	result, err := Compact(context.Background(), "previous", messages, writer, CompactOptions{
		MaxRecentTurns:         2,
		SummaryThresholdTokens: 1,
		TargetTokens:           1,
		ObservationKeepTurns:   2,
		SummaryIfAboveTokens:   1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if writer.called != 1 {
		t.Fatalf("expected LLM summary after masking was insufficient, called=%d", writer.called)
	}
	if !result.Changed {
		t.Fatal("expected compaction to change the session")
	}
	if result.Summary != "llm summary" {
		t.Fatalf("summary = %q", result.Summary)
	}
}

func TestParseObservationRefFromShortPlaceholder(t *testing.T) {
	id, path := ParseObservationRef("[observation-masked] tool=View observation_id=call-abc-123")
	if id != "call-abc-123" {
		t.Fatalf("id = %q", id)
	}
	if path != "" {
		t.Fatalf("path = %q, want empty", path)
	}

	id, path = ParseObservationRef("[observation-masked]\ntool=View tool_use_id=call-old observation_id=legacy-id\npath=/tmp/obs.txt")
	if id != "legacy-id" || path != "/tmp/obs.txt" {
		t.Fatalf("legacy parse id=%q path=%q", id, path)
	}
}

func TestObservationStoreDirNestsUnderSessionDir(t *testing.T) {
	got := ObservationStoreDir("/tmp/sessions", "main")
	if !strings.Contains(filepath.ToSlash(got), "/observations/main") {
		t.Fatalf("ObservationStoreDir = %q", got)
	}
}
