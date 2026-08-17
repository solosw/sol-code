package session

import (
	"context"
	"strings"
	"testing"
)

func TestFileStoreSessionLockPreventsSecondOwner(t *testing.T) {
	store := NewFileStore(t.TempDir())
	release, err := store.Acquire(context.Background(), "main")
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer release()

	if _, err := store.Acquire(context.Background(), "main"); err == nil || !strings.Contains(err.Error(), "already open") {
		t.Fatalf("second Acquire error = %v, want already open", err)
	}
	if err := release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if _, err := store.Acquire(context.Background(), "main"); err != nil {
		t.Fatalf("Acquire after release: %v", err)
	}
}
