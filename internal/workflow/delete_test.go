package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeleteWorkflowDir(t *testing.T) {
	root := t.TempDir()
	def := Definition{
		Name:        "to-delete",
		Description: "temp",
		Tasks:       []TaskSpec{{ID: "a", Description: "A", Prompt: "a"}},
	}
	path, err := SaveToDirWithLayout(def, root, &Layout{Nodes: map[string]NodePos{"a": {X: 1, Y: 2}}})
	if err != nil {
		t.Fatal(err)
	}
	def.Path = path
	if err := Delete(def, []string{root}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("expected directory removed, err=%v", err)
	}
}

func TestDeleteRefusesOutsideRoots(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	def := Definition{
		Name:        "outside",
		Description: "temp",
		Tasks:       []TaskSpec{{ID: "a", Description: "A", Prompt: "a"}},
	}
	path, err := SaveToDir(def, other)
	if err != nil {
		t.Fatal(err)
	}
	def.Path = path
	if err := Delete(def, []string{root}); err == nil {
		t.Fatal("expected refuse outside roots")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file should still exist: %v", err)
	}
}
