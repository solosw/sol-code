package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoalCheckerPromptOwnsChecklistUpdates(t *testing.T) {
	goalPath := filepath.Join(t.TempDir(), goalFileName)
	prompt := fmt.Sprintf(`You are the reusable goal-checking sub-agent for this project. %q is the authoritative feature checklist.

First inspect goal.md, the relevant implementation, and applicable tests. Then update goal.md yourself so it accurately reflects the independently verified state:
- Mark a checklist item [x] only when its implementation and validation evidence are present.
- Return any incorrectly checked or unsupported item to [ ].
- Add concrete missing feature or validation items when the existing checklist is incomplete or too vague.
- Keep the file concise; do not implement application features or change files other than goal.md.`, goalPath)
	for _, want := range []string{"update goal.md yourself", "implementation and validation evidence", "other than goal.md"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("goal checker prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestEnsureGoalFileCreatesGoalAlignedWithRequest(t *testing.T) {
	workDir := t.TempDir()
	path, err := EnsureGoalFile(workDir, "Add goal-driven execution")
	if err != nil {
		t.Fatalf("EnsureGoalFile: %v", err)
	}
	if path != filepath.Join(workDir, goalFileName) {
		t.Fatalf("path = %q", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read goal file: %v", err)
	}
	text := string(data)
	for _, want := range []string{"Add goal-driven execution", "## Completion checklist", "- [ ] Implement the requested goal."} {
		if !strings.Contains(text, want) {
			t.Fatalf("goal file missing %q:\n%s", want, text)
		}
	}
}

func TestEnsureGoalFileAddsNewRequestWithoutDiscardingExistingGoal(t *testing.T) {
	workDir := t.TempDir()
	path := filepath.Join(workDir, goalFileName)
	original := "# Goal\n\nExisting objective\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureGoalFile(workDir, "Additional objective"); err != nil {
		t.Fatalf("EnsureGoalFile: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "Existing objective") || !strings.Contains(text, "Additional objective") {
		t.Fatalf("goal file did not preserve and extend objectives:\n%s", text)
	}
}

func TestEnsureGoalFileRequiresDescriptionForMissingGoal(t *testing.T) {
	_, err := EnsureGoalFile(t.TempDir(), "")
	if err == nil || !strings.Contains(err.Error(), "provide a goal") {
		t.Fatalf("error = %v, want missing goal description error", err)
	}
}
