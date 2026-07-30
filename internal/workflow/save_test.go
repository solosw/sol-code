package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveToDirRoundTrip(t *testing.T) {
	dir := t.TempDir()
	def := Definition{
		Name:        "demo-flow",
		Description: "Demo",
		Tasks: []TaskSpec{
			{ID: "a", Description: "A", Prompt: "do a", Difficulty: "easy"},
			{ID: "b", Description: "B", Prompt: "do b", DependsOn: []string{"a"}},
		},
	}
	path, err := SaveToDir(def, dir)
	if err != nil {
		t.Fatalf("SaveToDir: %v", err)
	}
	want := filepath.Join(dir, "demo-flow", "workflow.yaml")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	loaded, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if loaded.Name != "demo-flow" || len(loaded.Tasks) != 2 {
		t.Fatalf("loaded = %#v", loaded)
	}
	if err := loaded.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestMarshalYAMLRejectsInvalid(t *testing.T) {
	_, err := MarshalYAML(Definition{Name: "x"})
	if err == nil {
		t.Fatal("expected error for empty tasks")
	}
}

func TestRenderGraphLevels(t *testing.T) {
	tasks := []TaskSpec{
		{ID: "code", Description: "c", Prompt: "c"},
		{ID: "docs", Description: "d", Prompt: "d"},
		{ID: "merge", Description: "m", Prompt: "m", DependsOn: []string{"code", "docs"}},
	}
	levels := TaskLevels(tasks)
	if len(levels) != 2 {
		t.Fatalf("levels = %#v", levels)
	}
	if len(levels[0]) != 2 || len(levels[1]) != 1 || levels[1][0] != "merge" {
		t.Fatalf("levels = %#v", levels)
	}
	graph := RenderGraph(tasks)
	if !strings.Contains(graph, "[code]") || !strings.Contains(graph, "[merge]") {
		t.Fatalf("graph = %q", graph)
	}
	// Ensure file exists after save helper creates parent dirs.
	if _, err := os.Stat(t.TempDir()); err != nil {
		t.Fatal(err)
	}
}
