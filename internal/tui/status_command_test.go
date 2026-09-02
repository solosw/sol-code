package tui

import (
	"strings"
	"testing"
)

func TestSlashStatusTextIncludesContextAndCache(t *testing.T) {
	m := Model{
		modelName:      "claude-sonnet",
		cwd:            "/tmp/proj",
		permissionMode: "plan",
		tokenUsage: TokenUsage{
			EstimatedContextTokens:   12_500,
			MaxContextTokens:         200_000,
			InputTokens:              1_000,
			OutputTokens:             250,
			CacheCreationInputTokens: 500,
			CacheReadInputTokens:     4_000,
		},
	}
	text := m.slashStatusText()
	for _, want := range []string{
		"Model: claude-sonnet",
		"Mode: plan",
		"Workdir: /tmp/proj",
		"Proxy:",
		"Context: 12.5k / 200k",
		"Cache:",
		"read 4k",
		"write 500",
		"Output: 250",
		"Input: 1k uncached",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("status text missing %q\n%s", want, text)
		}
	}
}

func TestSlashHelpListsStatus(t *testing.T) {
	if !strings.Contains(slashHelpText(), "/status") {
		t.Fatal("help should list /status")
	}
}

func TestBuiltinIncludesProxy(t *testing.T) {
	if !isBuiltinSlashCommand("proxy") {
		t.Fatal("proxy should be a builtin slash command")
	}
	if !strings.Contains(slashHelpText(), "/proxy") {
		t.Fatal("help should list /proxy")
	}
}

func TestBuiltinIncludesStatus(t *testing.T) {
	if !isBuiltinSlashCommand("status") {
		t.Fatal("status should be a builtin slash command")
	}
}
