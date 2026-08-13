package tui

import (
	"strings"
	"testing"
)

func TestSanitizeTerminalTextRemovesMarginAndCursorControls(t *testing.T) {
	input := "before\x1b[?69h\x1b[3;80s\x1b[2Dafter\x1b]8;;https://example.invalid\a link\x1b]8;;\a"
	if got := sanitizeTerminalText(input); got != "beforeafter link" {
		t.Fatalf("sanitizeTerminalText() = %q", got)
	}
}

func TestRenderMessagesDoesNotEmitToolControlSequences(t *testing.T) {
	rendered := renderMessages([]ChatMessage{{
		Role:     "tool-done",
		ToolName: "Bash",
		Content:  "\x1b[?69h\x1b[3;80soutput",
	}}, Dark, false, 80)
	if strings.Contains(rendered, "\x1b[?69h") || strings.Contains(rendered, "\x1b[3;80s") {
		t.Fatalf("rendered output retained terminal control sequence: %q", rendered)
	}
	if !strings.Contains(rendered, "output") {
		t.Fatalf("rendered output = %q, want text content", rendered)
	}
}
