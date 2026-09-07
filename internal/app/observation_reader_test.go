package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/solosw/solcode/internal/config"
	"github.com/solosw/solcode/internal/session"
	"github.com/solosw/solcode/internal/tool"
)

func TestAppReadObservationFromPlaceholder(t *testing.T) {
	sessionDir := t.TempDir()
	application := &App{Config: config.Config{Session: config.SessionConfig{Dir: sessionDir, DefaultSession: "main"}}}
	store := session.NewFileObservationStore(session.ObservationStoreDir(sessionDir, "main"))
	id := "toolu_old-abc"
	if _, err := store.Save(id, "original fetch body"); err != nil {
		t.Fatal(err)
	}
	placeholder := "[observation-masked] tool=Fetch observation_id=" + id

	got, err := application.ReadObservation(context.Background(), tool.ObservationReadRequest{
		Ref:       placeholder,
		SessionID: "main",
	})
	if err != nil {
		t.Fatalf("ReadObservation() = %v", err)
	}
	if got != "original fetch body" {
		t.Fatalf("got %q", got)
	}

	got, err = application.ReadObservation(context.Background(), tool.ObservationReadRequest{
		ID:        id,
		SessionID: "main",
	})
	if err != nil {
		t.Fatalf("ReadObservation(id) = %v", err)
	}
	if got != "original fetch body" {
		t.Fatalf("id lookup got %q", got)
	}
}

func TestAppReadObservationRejectsOutsidePath(t *testing.T) {
	sessionDir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	application := &App{Config: config.Config{Session: config.SessionConfig{Dir: sessionDir, DefaultSession: "main"}}}
	_, err := application.ReadObservation(context.Background(), tool.ObservationReadRequest{
		Path:      outside,
		SessionID: "main",
	})
	if err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("error = %v, want outside-store rejection", err)
	}
}
