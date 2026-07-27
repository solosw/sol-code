package unit_tests

import (
	"strings"
	"testing"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/solosw/solcode/internal/engine"
	"github.com/solosw/solcode/internal/tool"
)

// The default system prompt must tell the model that the memory tools exist and
// when to reach for them; otherwise the tools are registered but never used.
func TestDefaultSystemPromptDocumentsMemoryTools(t *testing.T) {
	builder := engine.ContextBuilder{}
	req := builder.Build(engine.BuildRequest{
		Messages: []sdk.MessageParam{sdk.NewUserMessage(sdk.NewTextBlock("hello"))},
	})
	for _, want := range []string{
		"# Memory",
		"WriteMemory",
		"ReadMemory",
		"cross-session memory",
	} {
		if !strings.Contains(req.System, want) {
			t.Fatalf("expected system prompt to document memory tools with %q, got %q", want, req.System)
		}
	}
}

func TestWriteMemoryDescriptionCoversWhenAndWhatToSave(t *testing.T) {
	desc := tool.NewWriteMemoryTool(nil).Description()
	for _, want := range []string{
		"durable",        // what qualifies
		"TodoWrite",      // where transient state belongs instead
		"secrets",        // rejected content
		"near-duplicate", // merge behavior
		"ReadMemory",     // how entries come back
	} {
		if !strings.Contains(desc, want) {
			t.Fatalf("WriteMemory description missing %q: %s", want, desc)
		}
	}
}

func TestReadMemoryDescriptionCoversWhenToLookUp(t *testing.T) {
	desc := tool.NewReadMemoryTool(nil).Description()
	for _, want := range []string{
		"WriteMemory",    // pairing
		"cross-session",  // visibility rule
		"other-session",  // result labeling
		"trust the code", // conflict resolution
	} {
		if !strings.Contains(desc, want) {
			t.Fatalf("ReadMemory description missing %q: %s", want, desc)
		}
	}
}
