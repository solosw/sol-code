package tui

import (
	"strings"
	"testing"

	"github.com/solosw/solcode/internal/workflow"
)

func TestSlashHelpIncludesWorkflowEdit(t *testing.T) {
	if !strings.Contains(slashHelpText(), "/workflow-edit") {
		t.Fatal("help missing /workflow-edit")
	}
	if !strings.Contains(slashHelpText(), "/web-ui") {
		t.Fatal("help missing /web-ui")
	}
	if !isBuiltinSlashCommand("workflow-edit") {
		t.Fatal("workflow-edit should be builtin")
	}
	if !isBuiltinSlashCommand("web-ui") {
		t.Fatal("web-ui should be builtin")
	}
}

func TestApplyWorkflowFieldAndDeps(t *testing.T) {
	def := workflow.Definition{
		Name:        "demo",
		Description: "d",
		Tasks: []workflow.TaskSpec{
			{ID: "a", Description: "A", Prompt: "pa"},
			{ID: "b", Description: "B", Prompt: "pb"},
		},
	}
	if err := applyWorkflowField(&def, 1, wfFieldTaskTools, "View, Grep"); err != nil {
		t.Fatal(err)
	}
	if len(def.Tasks[1].AllowedTools) != 2 {
		t.Fatalf("tools = %#v", def.Tasks[1].AllowedTools)
	}
	cands := dependencyCandidates(def, 1)
	if len(cands) != 1 || cands[0] != "a" {
		t.Fatalf("candidates = %#v", cands)
	}
}

func TestRenderGraphInEditorDraft(t *testing.T) {
	def := workflow.Definition{
		Name: "g",
		Tasks: []workflow.TaskSpec{
			{ID: "a", Description: "A", Prompt: "a"},
			{ID: "b", Description: "B", Prompt: "b", DependsOn: []string{"a"}},
		},
	}
	graph := workflow.RenderGraph(def.Tasks)
	if !strings.Contains(graph, "[a]") || !strings.Contains(graph, "[b]") {
		t.Fatalf("graph = %q", graph)
	}
}
