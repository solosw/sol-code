package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLayoutRoundTrip(t *testing.T) {
	dir := t.TempDir()
	def := Definition{
		Name:        "layout-demo",
		Description: "d",
		Tasks: []TaskSpec{
			{ID: "a", Description: "A", Prompt: "a"},
			{ID: "b", Description: "B", Prompt: "b", DependsOn: []string{"a"}},
		},
	}
	layout := Layout{Nodes: map[string]NodePos{"a": {X: 1, Y: 2}, "b": {X: 3, Y: 4}}}
	path, err := SaveToDirWithLayout(def, dir, &layout)
	if err != nil {
		t.Fatalf("SaveToDirWithLayout: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadLayout(path)
	if err != nil {
		t.Fatalf("LoadLayout: %v", err)
	}
	if loaded.Nodes["a"].X != 1 || loaded.Nodes["b"].Y != 4 {
		t.Fatalf("layout = %#v", loaded)
	}
	if filepath.Base(LayoutPath(path)) != "layout.json" {
		t.Fatalf("layout path = %q", LayoutPath(path))
	}
	auto := DefaultLayout(def)
	if len(auto.Nodes) != 2 {
		t.Fatalf("default layout = %#v", auto)
	}
}
