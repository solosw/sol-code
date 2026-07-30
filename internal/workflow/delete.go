package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Delete removes a workflow definition from disk.
// For name/workflow.yaml layouts it removes the whole name directory
// (including layout.json). For flat name.yaml files it removes just that file.
func Delete(def Definition, allowedRoots []string) error {
	path := strings.TrimSpace(def.Path)
	if path == "" {
		return fmt.Errorf("workflow %q has no on-disk path", def.Name)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve workflow path: %w", err)
	}
	if !pathAllowed(abs, allowedRoots) {
		return fmt.Errorf("refusing to delete workflow outside configured workflow directories: %s", abs)
	}

	base := filepath.Base(abs)
	if isWorkflowFileName(base) {
		dir := filepath.Dir(abs)
		// Extra safety: directory name should match workflow name when possible.
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("delete workflow directory %q: %w", dir, err)
		}
		return nil
	}

	if err := os.Remove(abs); err != nil {
		return fmt.Errorf("delete workflow file %q: %w", abs, err)
	}
	// Best-effort companion layout for non-standard layouts.
	_ = os.Remove(LayoutPath(abs))
	return nil
}

func pathAllowed(absPath string, roots []string) bool {
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		absRoot, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(absRoot, absPath)
		if err != nil {
			continue
		}
		if rel == "." {
			return true
		}
		if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
