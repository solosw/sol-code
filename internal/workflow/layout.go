package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Layout stores canvas positions for the web node editor.
type Layout struct {
	Nodes map[string]NodePos `json:"nodes,omitempty"`
}

// NodePos is a canvas coordinate for one task node.
type NodePos struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// LayoutPath returns the companion layout file next to a workflow.yaml path.
func LayoutPath(workflowPath string) string {
	dir := filepath.Dir(workflowPath)
	return filepath.Join(dir, "layout.json")
}

// LoadLayout reads layout.json if present.
func LoadLayout(workflowPath string) (Layout, error) {
	path := LayoutPath(workflowPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Layout{Nodes: map[string]NodePos{}}, nil
		}
		return Layout{}, err
	}
	var layout Layout
	if err := json.Unmarshal(data, &layout); err != nil {
		return Layout{}, fmt.Errorf("parse layout %q: %w", path, err)
	}
	if layout.Nodes == nil {
		layout.Nodes = map[string]NodePos{}
	}
	return layout, nil
}

// SaveLayout writes layout.json next to the workflow file.
func SaveLayout(workflowPath string, layout Layout) error {
	if layout.Nodes == nil {
		layout.Nodes = map[string]NodePos{}
	}
	data, err := json.MarshalIndent(layout, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	path := LayoutPath(workflowPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// SaveToDirWithLayout writes workflow.yaml and optional layout.json under dir/<name>/.
func SaveToDirWithLayout(def Definition, dir string, layout *Layout) (string, error) {
	path, err := SaveToDir(def, dir)
	if err != nil {
		return "", err
	}
	if layout != nil {
		if err := SaveLayout(path, *layout); err != nil {
			return path, fmt.Errorf("save layout: %w", err)
		}
	}
	return path, nil
}

// DefaultLayout places tasks in dependency levels left-to-right.
func DefaultLayout(def Definition) Layout {
	levels := TaskLevels(def.Tasks)
	nodes := map[string]NodePos{}
	const xGap, yGap = 280.0, 140.0
	for li, level := range levels {
		for ti, id := range level {
			nodes[id] = NodePos{
				X: 80 + float64(li)*xGap,
				Y: 80 + float64(ti)*yGap,
			}
		}
	}
	// Fallback for empty ids.
	for i, task := range def.Tasks {
		id := strings.TrimSpace(task.ID)
		if id == "" {
			id = fmt.Sprintf("task-%d", i+1)
		}
		if _, ok := nodes[id]; !ok {
			nodes[id] = NodePos{X: 80, Y: 80 + float64(i)*yGap}
		}
	}
	return Layout{Nodes: nodes}
}
