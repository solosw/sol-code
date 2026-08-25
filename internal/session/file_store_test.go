package session

import (
	"context"
	"strings"
	"testing"
)

func TestManualSessionSwitchIgnoresExistingLockAndKeepsOwnership(t *testing.T) {
	dir := t.TempDir()
	first := NewManager(NewFileStore(dir), "main")
	second := NewManager(NewFileStore(dir), "main")

	if _, _, err := first.LoadOrCreateAndAcquire(context.Background(), "main", dir, "test"); err != nil {
		t.Fatalf("startup acquire: %v", err)
	}
	if _, _, err := second.LoadOrCreateAndAcquireIgnoringExisting(context.Background(), "main", dir, "test"); err != nil {
		t.Fatalf("manual switch ignoring lock: %v", err)
	}
	if err := second.ReleaseAll(); err != nil {
		t.Fatalf("second ReleaseAll: %v", err)
	}
	third := NewManager(NewFileStore(dir), "main")
	if _, _, err := third.LoadOrCreateAndAcquire(context.Background(), "main", dir, "test"); err == nil || !strings.Contains(err.Error(), "already open") {
		t.Fatalf("startup while first still owns session error = %v, want already open", err)
	}
	if err := first.ReleaseAll(); err != nil {
		t.Fatalf("first ReleaseAll: %v", err)
	}
	if _, _, err := third.LoadOrCreateAndAcquire(context.Background(), "main", dir, "test"); err != nil {
		t.Fatalf("startup after all owners exit: %v", err)
	}
}

func TestManagerLoadOrCreateAndAcquireOwnsUntilReleaseAll(t *testing.T) {
	dir := t.TempDir()
	first := NewManager(NewFileStore(dir), "main")
	second := NewManager(NewFileStore(dir), "main")

	if _, _, err := first.LoadOrCreateAndAcquire(context.Background(), "main", dir, "test"); err != nil {
		t.Fatalf("first load and acquire: %v", err)
	}
	if _, _, err := second.LoadOrCreateAndAcquire(context.Background(), "main", dir, "test"); err == nil || !strings.Contains(err.Error(), "already open") {
		t.Fatalf("second load and acquire error = %v, want already open", err)
	}
	if err := first.ReleaseAll(); err != nil {
		t.Fatalf("ReleaseAll: %v", err)
	}
	if _, _, err := second.LoadOrCreateAndAcquire(context.Background(), "main", dir, "test"); err != nil {
		t.Fatalf("load and acquire after terminal release: %v", err)
	}
}

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

func TestManagerLoadMissingSession(t *testing.T) {
	manager := NewManager(NewFileStore(t.TempDir()), "main")
	_, err := manager.Load(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Fatalf("Load missing session error = %v, want not found", err)
	}
}
