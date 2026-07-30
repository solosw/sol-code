package unit_tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/solosw/solcode/internal/app"
	"github.com/solosw/solcode/internal/config"
	"github.com/solosw/solcode/internal/workflow"
)

func TestAppDeleteWorkflow(t *testing.T) {
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
		Name:        "doomed",
		Description: "will delete",
		Tasks:       []workflow.TaskSpec{{ID: "a", Description: "A", Prompt: "a"}},
	}
	path, err := application.SaveWorkflow(def, workflow.SaveScopeProject)
	if err != nil {
		t.Fatalf("SaveWorkflow: %v", err)
	}
	found := false
	for _, item := range application.ListWorkflows() {
		if item.Name == "doomed" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected saved workflow loaded")
	}
	if err := application.DeleteWorkflow("doomed"); err != nil {
		t.Fatalf("DeleteWorkflow: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected file gone, err=%v", err)
	}
	for _, item := range application.ListWorkflows() {
		if item.Name == "doomed" {
			t.Fatal("workflow still listed after delete")
		}
	}
	// sanity: project dir path shape
	_ = filepath.Join(config.ProjectConfigDir(work), "workflows")
}
