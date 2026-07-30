package workflow

import (
	"fmt"
	"sort"
	"strings"
)

// TaskLevels groups tasks into dependency layers for visualization.
// Independent tasks share a level; dependents appear after their prerequisites.
func TaskLevels(tasks []TaskSpec) [][]string {
	if len(tasks) == 0 {
		return nil
	}
	ids := make([]string, 0, len(tasks))
	byID := make(map[string]TaskSpec, len(tasks))
	for i, task := range tasks {
		id := strings.TrimSpace(task.ID)
		if id == "" {
			id = fmt.Sprintf("task-%d", i+1)
		}
		task.ID = id
		ids = append(ids, id)
		byID[id] = task
	}

	levelOf := make(map[string]int, len(ids))
	remaining := make(map[string]TaskSpec, len(byID))
	for id, task := range byID {
		remaining[id] = task
	}
	for len(remaining) > 0 {
		ready := make([]string, 0)
		for id, task := range remaining {
			level := 0
			ok := true
			for _, dep := range task.DependsOn {
				dep = strings.TrimSpace(dep)
				if dep == "" {
					continue
				}
				if _, known := byID[dep]; !known {
					continue
				}
				if _, done := levelOf[dep]; !done {
					ok = false
					break
				}
				if levelOf[dep]+1 > level {
					level = levelOf[dep] + 1
				}
			}
			if ok {
				levelOf[id] = level
				ready = append(ready, id)
			}
		}
		if len(ready) == 0 {
			// Cycle or unresolved deps: dump leftovers as a final level.
			left := make([]string, 0, len(remaining))
			for id := range remaining {
				left = append(left, id)
			}
			sort.Strings(left)
			maxLevel := 0
			for _, lvl := range levelOf {
				if lvl+1 > maxLevel {
					maxLevel = lvl + 1
				}
			}
			for _, id := range left {
				levelOf[id] = maxLevel
			}
			break
		}
		for _, id := range ready {
			delete(remaining, id)
		}
	}

	maxLevel := 0
	for _, lvl := range levelOf {
		if lvl > maxLevel {
			maxLevel = lvl
		}
	}
	levels := make([][]string, maxLevel+1)
	for _, id := range ids {
		lvl := levelOf[id]
		levels[lvl] = append(levels[lvl], id)
	}
	for i := range levels {
		sort.Strings(levels[i])
	}
	return levels
}

// RenderGraph returns a compact ASCII DAG of workflow tasks.
func RenderGraph(tasks []TaskSpec) string {
	levels := TaskLevels(tasks)
	if len(levels) == 0 {
		return "(no tasks)"
	}
	byID := make(map[string]TaskSpec, len(tasks))
	for i, task := range tasks {
		id := strings.TrimSpace(task.ID)
		if id == "" {
			id = fmt.Sprintf("task-%d", i+1)
		}
		task.ID = id
		byID[id] = task
	}

	var b strings.Builder
	for i, level := range levels {
		nodes := make([]string, 0, len(level))
		for _, id := range level {
			nodes = append(nodes, "["+id+"]")
		}
		b.WriteString(strings.Join(nodes, "   "))
		if i < len(levels)-1 {
			b.WriteString("\n")
			// Draw dependency arrows into the next level when possible.
			next := levels[i+1]
			arrows := make([]string, 0, len(next))
			for _, id := range next {
				deps := make([]string, 0)
				for _, dep := range byID[id].DependsOn {
					dep = strings.TrimSpace(dep)
					if dep == "" {
						continue
					}
					for _, prev := range level {
						if dep == prev {
							deps = append(deps, dep)
						}
					}
				}
				if len(deps) == 0 {
					arrows = append(arrows, "  │")
					continue
				}
				arrows = append(arrows, "  ▲ ← "+strings.Join(deps, ","))
			}
			b.WriteString(strings.Join(arrows, "\n"))
			b.WriteString("\n")
		}
	}
	return b.String()
}
