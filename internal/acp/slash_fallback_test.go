package acp

import (
	"context"
	"testing"

	"github.com/solosw/solcode/internal/app"
	"github.com/solosw/solcode/internal/config"
)

func TestParseSlashCommandRejectsDoubleSlash(t *testing.T) {
	for _, in := range []string{"/", "//", "// comment", "//path", "hello", "/tmp/x", "/-x"} {
		if cmd, ok := parseSlashCommand(in); ok {
			t.Fatalf("parseSlashCommand(%q)=%#v, want not ok", in, cmd)
		}
	}
	cmd, ok := parseSlashCommand("/help extra")
	if !ok || cmd.Name != "help" || cmd.Args != "extra" {
		t.Fatalf("got %#v ok=%v", cmd, ok)
	}
}

func TestHandleSlashUnknownFallsThrough(t *testing.T) {
	s := &Server{}
	sess := &acpSession{
		id:          "s1",
		application: &app.App{},
		cfg:         config.Config{},
	}
	handled, _, err := s.handleSlashCommand(context.Background(), sess, "// not a command")
	if err != nil {
		t.Fatal(err)
	}
	if handled {
		t.Fatal("// should fall through to agent chat")
	}
	handled, _, err = s.handleSlashCommand(context.Background(), sess, "/not-a-real-command")
	if err != nil {
		t.Fatal(err)
	}
	if handled {
		t.Fatal("unknown slash token should fall through to agent chat")
	}
	handled, _, err = s.handleSlashCommand(context.Background(), sess, "/help")
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("/help should be handled")
	}
}
