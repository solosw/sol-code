package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// SaveScope selects the destination root for a workflow definition.
type SaveScope string

const (
	SaveScopeUser    SaveScope = "user"
	SaveScopeProject SaveScope = "project"
)

// MarshalYAML encodes a definition for on-disk authoring.
func MarshalYAML(def Definition) ([]byte, error) {
	out := Definition{
		Name:          normalizeName(def.Name),
		Description:   strings.TrimSpace(def.Description),
		ExecutionMode: strings.TrimSpace(def.ExecutionMode),
		Tasks:         cloneTasks(def.Tasks),
	}
	if out.Name == "" {
		return nil, fmt.Errorf("workflow name is required")
	}
	if err := out.Validate(); err != nil {
		return nil, err
	}
	data, err := yaml.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("encode workflow yaml: %w", err)
	}
	return data, nil
}

// SaveToDir writes def as <dir>/<name>/workflow.yaml and returns the file path.
func SaveToDir(def Definition, dir string) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", fmt.Errorf("workflow directory is required")
	}
	name := normalizeName(def.Name)
	if name == "" {
		return "", fmt.Errorf("workflow name is required")
	}
	def.Name = name
	data, err := MarshalYAML(def)
	if err != nil {
		return "", err
	}
	targetDir := filepath.Join(dir, name)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("create workflow directory %q: %w", targetDir, err)
	}
	path := filepath.Join(targetDir, "workflow.yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write workflow %q: %w", path, err)
	}
	return path, nil
}

func cloneTasks(tasks []TaskSpec) []TaskSpec {
	if len(tasks) == 0 {
		return nil
	}
	out := make([]TaskSpec, 0, len(tasks))
	for _, task := range tasks {
		out = append(out, TaskSpec{
			ID:           strings.TrimSpace(task.ID),
			Description:  strings.TrimSpace(task.Description),
			Prompt:       task.Prompt,
			AllowedTools: append([]string(nil), task.AllowedTools...),
			Difficulty:   strings.TrimSpace(task.Difficulty),
			Model:        strings.TrimSpace(task.Model),
			DependsOn:    append([]string(nil), task.DependsOn...),
		})
	}
	return out
}
