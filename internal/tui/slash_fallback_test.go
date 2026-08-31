package tui

import "testing"

func TestParseSlashCommandRejectsDoubleSlashAndBareSlash(t *testing.T) {
	cases := []string{
		"",
		"hello",
		"/",
		"//",
		"// comment",
		"//path/to/file",
		" / ",
	}
	for _, in := range cases {
		if cmd, ok := parseSlashCommand(in); ok {
			t.Fatalf("parseSlashCommand(%q) = %#v, want not ok", in, cmd)
		}
	}
}

func TestParseSlashCommandAcceptsBuiltins(t *testing.T) {
	cmd, ok := parseSlashCommand("/help")
	if !ok || cmd.Name != "help" || cmd.Args != "" {
		t.Fatalf("got %#v ok=%v", cmd, ok)
	}
	cmd, ok = parseSlashCommand("/new-session my-session")
	if !ok || cmd.Name != "new-session" || cmd.Args != "my-session" {
		t.Fatalf("got %#v ok=%v", cmd, ok)
	}
}

func TestParseSlashCommandRejectsInvalidTokens(t *testing.T) {
	// Looks like a path or prose, not a command token.
	for _, in := range []string{"/tmp/foo", "/C:\\Windows", "/你好", "/-bad"} {
		if cmd, ok := parseSlashCommand(in); ok {
			t.Fatalf("parseSlashCommand(%q) = %#v, want not ok", in, cmd)
		}
	}
}

func TestHandleSlashUnknownFallsThroughToChat(t *testing.T) {
	m := Model{}
	// Well-formed but unknown command name → chat, not "Unknown command".
	if handled, _ := m.handleSlashCommand("/not-a-real-command please help"); handled {
		t.Fatal("unknown slash token should fall through as chat")
	}
	if handled, _ := m.handleSlashCommand("// still chat"); handled {
		t.Fatal("// should fall through as chat")
	}
	if handled, _ := m.handleSlashCommand("/tmp/path"); handled {
		t.Fatal("path-like input should fall through as chat")
	}
}

func TestHandleSlashHelpStillHandled(t *testing.T) {
	m := Model{}
	handled, _ := m.handleSlashCommand("/help")
	if !handled {
		t.Fatal("/help should be handled")
	}
	if len(m.messages) == 0 {
		t.Fatal("expected user+result messages for /help")
	}
}
