package unit_tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/solosw/solcode/internal/app"
	"github.com/solosw/solcode/internal/config"
	"github.com/solosw/solcode/internal/workflow"
)

func TestAppSaveWorkflowProjectScope(t *testing.T) {
	work := t.TempDir()
	cfg := config.Config{WorkDir: work}
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	application, err := app.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = application.Close() })

	def := workflow.Definition{
		Name:        "saved-flow",
		Description: "saved from editor",
		Tasks: []workflow.TaskSpec{
			{ID: "a", Description: "A", Prompt: "do a"},
		},
	}
	path, err := application.SaveWorkflow(def, workflow.SaveScopeProject)
	if err != nil {
		t.Fatalf("SaveWorkflow: %v", err)
	}
	wantDir := filepath.Join(config.ProjectConfigDir(work), "workflows", "saved-flow", "workflow.yaml")
	if path != wantDir {
		t.Fatalf("path = %q, want %q", path, wantDir)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat saved file: %v", err)
	}
	defs := application.ListWorkflows()
	found := false
	for _, item := range defs {
		if item.Name == "saved-flow" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("saved workflow not loaded: %#v", defs)
	}
}
