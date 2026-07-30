package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/solosw/solcode/internal/workflow"
)

type workflowEditorMode int

const (
	wfModeBrowse workflowEditorMode = iota
	wfModeEdit
	wfModeField
	wfModeDeps
	wfModeSaveScope
)

type workflowField int

const (
	wfFieldName workflowField = iota
	wfFieldDescription
	wfFieldTaskID
	wfFieldTaskDescription
	wfFieldTaskPrompt
	wfFieldTaskDifficulty
	wfFieldTaskTools
)

// WorkflowEditorCallbacks wires the TUI editor to app load/save.
type WorkflowEditorCallbacks struct {
	List       func() []workflow.Definition
	Save       func(def workflow.Definition, scope workflow.SaveScope) (string, error)
	Delete     func(name string) error
	UserDir    func() string
	ProjectDir func() string
}

// WorkflowEditorState is the interactive visual workflow orchestrator.
type WorkflowEditorState struct {
	mode     workflowEditorMode
	items    []workflow.Definition
	selected int

	draft     workflow.Definition
	taskIndex int
	dirty     bool
	status    string

	field          workflowField
	input          textinput.Model
	depCursor      int
	scopeSel       int // 0=user, 1=project
	confirmDiscard bool
	confirmDelete  bool
}

func (m *Model) SetWorkflowEditorCallbacks(cb WorkflowEditorCallbacks) {
	m.workflowEditorCallbacks = &cb
}

// SetWorkflowUIHandler registers the callback that opens the web node editor.
func (m *Model) SetWorkflowUIHandler(handler func() string) {
	m.workflowUIHandler = handler
}

func (m *Model) ShowWorkflowEditor() {
	if m.workflowEditorCallbacks == nil || m.workflowEditorCallbacks.List == nil {
		m.appendCommandResult("/workflow-edit is not available in this session.")
		return
	}
	items := m.workflowEditorCallbacks.List()
	input := textinput.New()
	input.CharLimit = 4000
	input.Width = max(20, m.width-12)
	m.workflowEditor = &WorkflowEditorState{
		mode:  wfModeBrowse,
		items: items,
		input: input,
	}
	m.status = "Workflow editor"
	m.resize()
	m.refreshViewport()
}

func (m *Model) closeWorkflowEditor(message string) {
	m.workflowEditor = nil
	if strings.TrimSpace(message) != "" {
		m.appendCommandResult(message)
	}
	m.status = "Ready"
	m.resize()
	m.refreshViewport()
}

func (m Model) handleWorkflowEditorKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	ed := m.workflowEditor
	if ed == nil {
		return m, nil
	}
	key := msg.String()
	switch ed.mode {
	case wfModeBrowse:
		return m.handleWorkflowBrowseKey(key)
	case wfModeEdit:
		return m.handleWorkflowEditKey(key)
	case wfModeField:
		return m.handleWorkflowFieldKey(msg, key)
	case wfModeDeps:
		return m.handleWorkflowDepsKey(key)
	case wfModeSaveScope:
		return m.handleWorkflowSaveScopeKey(key)
	default:
		return m, nil
	}
}

func (m Model) handleWorkflowBrowseKey(key string) (tea.Model, tea.Cmd) {
	ed := m.workflowEditor
	switch key {
	case "esc", "ctrl+c", "q":
		m.closeWorkflowEditor("")
	case "up", "k":
		ed.confirmDelete = false
		if ed.selected > 0 {
			ed.selected--
		}
	case "down", "j":
		ed.confirmDelete = false
		if ed.selected < len(ed.items)-1 {
			ed.selected++
		}
	case "n":
		ed.confirmDelete = false
		m.startWorkflowDraft(workflow.Definition{
			Name:        "new-workflow",
			Description: "Describe this workflow",
			Tasks: []workflow.TaskSpec{{
				ID:          "step-1",
				Description: "First step",
				Prompt:      "Do the first step. Args: {{args}}",
				Difficulty:  "easy",
			}},
		}, true)
	case "x", "delete", "backspace":
		if len(ed.items) == 0 {
			ed.status = "No workflow to delete"
			break
		}
		name := ed.items[ed.selected].Name
		if !ed.confirmDelete {
			ed.confirmDelete = true
			ed.status = fmt.Sprintf("Delete %q from disk? Press x again to confirm", name)
			break
		}
		ed.confirmDelete = false
		if m.workflowEditorCallbacks == nil || m.workflowEditorCallbacks.Delete == nil {
			ed.status = "Delete is not available"
			break
		}
		if err := m.workflowEditorCallbacks.Delete(name); err != nil {
			ed.status = err.Error()
			break
		}
		if m.workflowEditorCallbacks.List != nil {
			ed.items = m.workflowEditorCallbacks.List()
		} else {
			ed.items = nil
		}
		if ed.selected >= len(ed.items) && ed.selected > 0 {
			ed.selected = len(ed.items) - 1
		}
		if len(ed.items) == 0 {
			ed.selected = 0
		}
		ed.status = fmt.Sprintf("Deleted workflow %q", name)
	case "enter":
		ed.confirmDelete = false
		if len(ed.items) == 0 {
			return m, nil
		}
		def := ed.items[ed.selected]
		m.startWorkflowDraft(cloneWorkflowDefinition(def), false)
	}
	m.resize()
	m.refreshViewport()
	return m, nil
}

func (m *Model) startWorkflowDraft(def workflow.Definition, dirty bool) {
	ed := m.workflowEditor
	if ed == nil {
		return
	}
	if len(def.Tasks) == 0 {
		def.Tasks = []workflow.TaskSpec{{
			ID:          "step-1",
			Description: "First step",
			Prompt:      "Do the work. Args: {{args}}",
			Difficulty:  "easy",
		}}
		dirty = true
	}
	ed.draft = def
	ed.taskIndex = 0
	ed.dirty = dirty
	ed.mode = wfModeEdit
	ed.status = ""
}

func (m Model) handleWorkflowEditKey(key string) (tea.Model, tea.Cmd) {
	ed := m.workflowEditor
	switch key {
	case "esc", "q":
		if ed.dirty && !ed.confirmDiscard {
			ed.confirmDiscard = true
			ed.status = "Unsaved changes — press Esc again to discard, or s to save"
			break
		}
		ed.confirmDiscard = false
		ed.mode = wfModeBrowse
		if m.workflowEditorCallbacks != nil && m.workflowEditorCallbacks.List != nil {
			ed.items = m.workflowEditorCallbacks.List()
		}
		ed.selected = 0
		ed.dirty = false
		ed.status = ""
	case "up", "k":
		ed.confirmDiscard = false
		if ed.taskIndex > 0 {
			ed.taskIndex--
		}
	case "down", "j":
		ed.confirmDiscard = false
		if ed.taskIndex < len(ed.draft.Tasks)-1 {
			ed.taskIndex++
		}
	case "a":
		ed.confirmDiscard = false
		ed.draft.Tasks = append(ed.draft.Tasks, workflow.TaskSpec{
			ID:          nextTaskID(ed.draft.Tasks),
			Description: "New step",
			Prompt:      "Describe what this step should do. Args: {{args}}",
			Difficulty:  "easy",
		})
		ed.taskIndex = len(ed.draft.Tasks) - 1
		ed.dirty = true
	case "x", "delete", "backspace":
		if len(ed.draft.Tasks) <= 1 {
			ed.status = "Keep at least one task"
			break
		}
		removed := ed.draft.Tasks[ed.taskIndex].ID
		ed.draft.Tasks = append(ed.draft.Tasks[:ed.taskIndex], ed.draft.Tasks[ed.taskIndex+1:]...)
		// Drop dangling depends_on references.
		for i := range ed.draft.Tasks {
			ed.draft.Tasks[i].DependsOn = removeStringValue(ed.draft.Tasks[i].DependsOn, removed)
		}
		if ed.taskIndex >= len(ed.draft.Tasks) {
			ed.taskIndex = len(ed.draft.Tasks) - 1
		}
		ed.dirty = true
	case "e":
		m.beginWorkflowField(wfFieldTaskID)
	case "p":
		m.beginWorkflowField(wfFieldTaskPrompt)
	case "t":
		m.beginWorkflowField(wfFieldTaskTools)
	case "f":
		m.beginWorkflowField(wfFieldTaskDifficulty)
	case "c":
		m.beginWorkflowField(wfFieldTaskDescription)
	case "N":
		m.beginWorkflowField(wfFieldName)
	case "D":
		m.beginWorkflowField(wfFieldDescription)
	case "d":
		if len(ed.draft.Tasks) < 2 {
			ed.status = "Need at least two tasks to set dependencies"
			break
		}
		ed.mode = wfModeDeps
		ed.depCursor = 0
		ed.status = "Toggle dependencies with Space/Enter"
	case "m":
		ed.draft.ExecutionMode = nextExecutionMode(ed.draft.ExecutionMode)
		ed.dirty = true
	case "s":
		ed.confirmDiscard = false
		if err := ed.draft.Validate(); err != nil {
			ed.status = err.Error()
			break
		}
		ed.mode = wfModeSaveScope
		ed.scopeSel = 1 // prefer project
		ed.status = ""
	}
	m.resize()
	m.refreshViewport()
	return m, nil
}

func (m *Model) beginWorkflowField(field workflowField) {
	ed := m.workflowEditor
	if ed == nil {
		return
	}
	ed.field = field
	ed.mode = wfModeField
	ed.input.SetValue(workflowFieldValue(ed.draft, ed.taskIndex, field))
	ed.input.CursorEnd()
	ed.input.Focus()
	ed.input.Width = max(20, m.width-12)
	ed.status = workflowFieldLabel(field)
}

func (m Model) handleWorkflowFieldKey(msg tea.KeyMsg, key string) (tea.Model, tea.Cmd) {
	ed := m.workflowEditor
	switch key {
	case "esc":
		ed.input.Blur()
		ed.mode = wfModeEdit
		ed.status = ""
	case "enter":
		value := ed.input.Value()
		if err := applyWorkflowField(&ed.draft, ed.taskIndex, ed.field, value); err != nil {
			ed.status = err.Error()
			break
		}
		ed.dirty = true
		ed.input.Blur()
		ed.mode = wfModeEdit
		ed.status = "Updated"
	default:
		var cmd tea.Cmd
		ed.input, cmd = ed.input.Update(msg)
		m.resize()
		m.refreshViewport()
		return m, cmd
	}
	m.resize()
	m.refreshViewport()
	return m, nil
}

func (m Model) handleWorkflowDepsKey(key string) (tea.Model, tea.Cmd) {
	ed := m.workflowEditor
	candidates := dependencyCandidates(ed.draft, ed.taskIndex)
	switch key {
	case "esc", "q":
		ed.mode = wfModeEdit
		ed.status = ""
	case "up", "k":
		if ed.depCursor > 0 {
			ed.depCursor--
		}
	case "down", "j":
		if ed.depCursor < len(candidates)-1 {
			ed.depCursor++
		}
	case " ", "enter":
		if len(candidates) == 0 {
			break
		}
		id := candidates[ed.depCursor]
		task := &ed.draft.Tasks[ed.taskIndex]
		if containsString(task.DependsOn, id) {
			task.DependsOn = removeStringValue(task.DependsOn, id)
		} else {
			task.DependsOn = append(task.DependsOn, id)
		}
		ed.dirty = true
	case "done", "s":
		ed.mode = wfModeEdit
		ed.status = "Dependencies updated"
	}
	m.resize()
	m.refreshViewport()
	return m, nil
}

func (m Model) handleWorkflowSaveScopeKey(key string) (tea.Model, tea.Cmd) {
	ed := m.workflowEditor
	switch key {
	case "esc":
		ed.mode = wfModeEdit
	case "up", "k", "left", "h":
		ed.scopeSel = 0
	case "down", "j", "right", "l":
		ed.scopeSel = 1
	case "1":
		ed.scopeSel = 0
		return m.saveWorkflowDraft()
	case "2":
		ed.scopeSel = 1
		return m.saveWorkflowDraft()
	case "enter":
		return m.saveWorkflowDraft()
	}
	m.resize()
	m.refreshViewport()
	return m, nil
}

func (m Model) saveWorkflowDraft() (tea.Model, tea.Cmd) {
	ed := m.workflowEditor
	if ed == nil || m.workflowEditorCallbacks == nil || m.workflowEditorCallbacks.Save == nil {
		m.closeWorkflowEditor("Workflow save is not available.")
		return m, nil
	}
	scope := workflow.SaveScopeUser
	if ed.scopeSel == 1 {
		scope = workflow.SaveScopeProject
	}
	path, err := m.workflowEditorCallbacks.Save(ed.draft, scope)
	if err != nil {
		ed.mode = wfModeEdit
		ed.status = err.Error()
		m.resize()
		m.refreshViewport()
		return m, nil
	}
	msg := fmt.Sprintf("Saved workflow %q to %s", ed.draft.Name, path)
	// Stay open in browse with refreshed list.
	items := []workflow.Definition{}
	if m.workflowEditorCallbacks.List != nil {
		items = m.workflowEditorCallbacks.List()
	}
	ed.mode = wfModeBrowse
	ed.items = items
	ed.dirty = false
	ed.status = msg
	// Also echo into transcript.
	m.appendCommandResult(msg)
	m.resize()
	m.refreshViewport()
	return m, nil
}

func (m Model) renderWorkflowEditor() string {
	ed := m.workflowEditor
	if ed == nil {
		return ""
	}
	t := m.theme
	dialogWidth := min(max(48, m.width-4), 96)
	var body string
	switch ed.mode {
	case wfModeBrowse:
		body = m.renderWorkflowBrowse(dialogWidth)
	case wfModeEdit:
		body = m.renderWorkflowEdit(dialogWidth)
	case wfModeField:
		body = m.renderWorkflowField(dialogWidth)
	case wfModeDeps:
		body = m.renderWorkflowDeps(dialogWidth)
	case wfModeSaveScope:
		body = m.renderWorkflowSaveScope(dialogWidth)
	}
	return m.fitOverlay(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, t.DialogBorder.Width(dialogWidth).Render(body)))
}

func (m Model) renderWorkflowBrowse(width int) string {
	ed := m.workflowEditor
	t := m.theme
	title := t.PermTitle.Render("Workflow Orchestrator")
	if len(ed.items) == 0 {
		hint := t.PermHint.Render("[n] New workflow   [Esc] Close")
		empty := t.Muted.Render("No workflows loaded yet.")
		return strings.Join([]string{title, "", empty, "", hint}, "\n")
	}
	maxLines := m.maxOverlayListLines()
	start, end := visibleRange(ed.selected, len(ed.items), maxLines)
	var lines []string
	for i := start; i < end; i++ {
		item := ed.items[i]
		label := item.Name
		line := "  " + label
		if i == ed.selected {
			line = t.ClaudeStyle.Render("❯ ") + t.ClaudeStyle.Render(label)
		}
		subtitle := strings.TrimSpace(item.Description)
		if subtitle == "" {
			subtitle = fmt.Sprintf("%d tasks", len(item.Tasks))
		} else {
			subtitle = fmt.Sprintf("%s · %d tasks", truncate(subtitle, 40), len(item.Tasks))
		}
		line += "  " + t.Dim.Render(subtitle)
		lines = append(lines, line)
	}
	listWidth := max(16, width-6)
	list := m.renderScrollableList(lines, len(ed.items), start, maxLines, listWidth)
	hint := t.PermHint.Render("[↑/↓] Select  [Enter] Edit  [n] New  [x] Delete  [Esc] Close")
	if ed.status != "" {
		return strings.Join([]string{title, list, t.Muted.Render(ed.status), hint}, "\n")
	}
	return strings.Join([]string{title, list, hint}, "\n")
}

func (m Model) renderWorkflowEdit(width int) string {
	ed := m.workflowEditor
	t := m.theme
	dirty := ""
	if ed.dirty {
		dirty = t.Dim.Render(" *")
	}
	title := t.PermTitle.Render("Edit Workflow: ") + t.ClaudeStyle.Render(ed.draft.Name) + dirty
	desc := t.Muted.Render(truncate(strings.TrimSpace(ed.draft.Description), width-8))
	mode := ed.draft.ExecutionMode
	if mode == "" {
		mode = "auto"
	}
	meta := t.Dim.Render(fmt.Sprintf("execution_mode: %s", mode))
	graphTitle := t.Muted.Render("Graph")
	graph := t.ClaudeStyle.Render(workflow.RenderGraph(ed.draft.Tasks))
	tasksTitle := t.Muted.Render("Tasks")
	var taskLines []string
	for i, task := range ed.draft.Tasks {
		id := task.ID
		if id == "" {
			id = fmt.Sprintf("task-%d", i+1)
		}
		deps := "-"
		if len(task.DependsOn) > 0 {
			deps = strings.Join(task.DependsOn, ",")
		}
		tools := "-"
		if len(task.AllowedTools) > 0 {
			tools = strings.Join(task.AllowedTools, ",")
		}
		diff := task.Difficulty
		if diff == "" {
			diff = "default"
		}
		summary := fmt.Sprintf("%s — %s", id, truncate(task.Description, 36))
		detail := fmt.Sprintf("[%s] tools:%s deps:%s", diff, tools, deps)
		if i == ed.taskIndex {
			taskLines = append(taskLines, t.ClaudeStyle.Render("❯ ")+t.ClaudeStyle.Render(summary)+"  "+t.Dim.Render(detail))
		} else {
			taskLines = append(taskLines, "  "+summary+"  "+t.Dim.Render(detail))
		}
	}
	hint := t.PermHint.Render("[↑/↓] Task  [a] Add  [x] Del  [e] Edit  [d] Deps  [p] Prompt  [N/D] Name/Desc  [m] Mode  [s] Save  [Esc] Back")
	parts := []string{title, desc, meta, "", graphTitle, graph, "", tasksTitle, strings.Join(taskLines, "\n"), ""}
	if ed.status != "" {
		parts = append(parts, t.Muted.Render(ed.status), "")
	}
	parts = append(parts, hint)
	return strings.Join(parts, "\n")
}

func (m Model) renderWorkflowField(width int) string {
	ed := m.workflowEditor
	t := m.theme
	title := t.PermTitle.Render(workflowFieldLabel(ed.field))
	hint := t.PermHint.Render("[Enter] Apply  [Esc] Cancel")
	return strings.Join([]string{title, "", "  " + ed.input.View(), "", hint}, "\n")
}

func (m Model) renderWorkflowDeps(width int) string {
	ed := m.workflowEditor
	t := m.theme
	taskID := ed.draft.Tasks[ed.taskIndex].ID
	title := t.PermTitle.Render(fmt.Sprintf("Dependencies for %s", taskID))
	candidates := dependencyCandidates(ed.draft, ed.taskIndex)
	if len(candidates) == 0 {
		return strings.Join([]string{title, "", t.Muted.Render("No other tasks available."), "", t.PermHint.Render("[Esc] Back")}, "\n")
	}
	var lines []string
	for i, id := range candidates {
		mark := "[ ]"
		if containsString(ed.draft.Tasks[ed.taskIndex].DependsOn, id) {
			mark = "[x]"
		}
		line := fmt.Sprintf("  %s %s", mark, id)
		if i == ed.depCursor {
			line = t.ClaudeStyle.Render("❯ " + mark + " " + id)
		}
		lines = append(lines, line)
	}
	hint := t.PermHint.Render("[↑/↓] Move  [Space/Enter] Toggle  [Esc] Done")
	return strings.Join([]string{title, "", strings.Join(lines, "\n"), "", hint}, "\n")
}

func (m Model) renderWorkflowSaveScope(width int) string {
	ed := m.workflowEditor
	t := m.theme
	title := t.PermTitle.Render("Save Workflow")
	userPath := "~/.solcode/workflows/" + ed.draft.Name + "/workflow.yaml"
	projectPath := ".solcode/workflows/" + ed.draft.Name + "/workflow.yaml"
	if m.workflowEditorCallbacks != nil {
		if m.workflowEditorCallbacks.UserDir != nil {
			if dir := m.workflowEditorCallbacks.UserDir(); dir != "" {
				userPath = dir + "/" + ed.draft.Name + "/workflow.yaml"
			}
		}
		if m.workflowEditorCallbacks.ProjectDir != nil {
			if dir := m.workflowEditorCallbacks.ProjectDir(); dir != "" {
				projectPath = dir + "/" + ed.draft.Name + "/workflow.yaml"
			}
		}
	}
	options := []string{
		fmt.Sprintf("User directory\n    %s", userPath),
		fmt.Sprintf("Project directory\n    %s", projectPath),
	}
	var lines []string
	for i, opt := range options {
		prefix := "  "
		if i == ed.scopeSel {
			prefix = t.ClaudeStyle.Render("❯ ")
			lines = append(lines, prefix+t.ClaudeStyle.Render(strings.Split(opt, "\n")[0]))
			rest := strings.SplitN(opt, "\n", 2)
			if len(rest) > 1 {
				lines = append(lines, "    "+t.Dim.Render(strings.TrimSpace(rest[1])))
			}
			continue
		}
		parts := strings.SplitN(opt, "\n", 2)
		lines = append(lines, prefix+parts[0])
		if len(parts) > 1 {
			lines = append(lines, "    "+t.Dim.Render(strings.TrimSpace(parts[1])))
		}
	}
	hint := t.PermHint.Render("[↑/↓] Choose  [Enter] Save  [1] User  [2] Project  [Esc] Cancel")
	return strings.Join([]string{title, "", strings.Join(lines, "\n"), "", hint}, "\n")
}

func workflowFieldLabel(field workflowField) string {
	switch field {
	case wfFieldName:
		return "Workflow name"
	case wfFieldDescription:
		return "Workflow description"
	case wfFieldTaskID:
		return "Task id"
	case wfFieldTaskDescription:
		return "Task description"
	case wfFieldTaskPrompt:
		return "Task prompt"
	case wfFieldTaskDifficulty:
		return "Task difficulty (easy|medium|hard)"
	case wfFieldTaskTools:
		return "Allowed tools (comma-separated)"
	default:
		return "Edit field"
	}
}

func workflowFieldValue(def workflow.Definition, taskIndex int, field workflowField) string {
	switch field {
	case wfFieldName:
		return def.Name
	case wfFieldDescription:
		return def.Description
	}
	if taskIndex < 0 || taskIndex >= len(def.Tasks) {
		return ""
	}
	task := def.Tasks[taskIndex]
	switch field {
	case wfFieldTaskID:
		return task.ID
	case wfFieldTaskDescription:
		return task.Description
	case wfFieldTaskPrompt:
		return task.Prompt
	case wfFieldTaskDifficulty:
		return task.Difficulty
	case wfFieldTaskTools:
		return strings.Join(task.AllowedTools, ", ")
	default:
		return ""
	}
}

func applyWorkflowField(def *workflow.Definition, taskIndex int, field workflowField, value string) error {
	value = strings.TrimSpace(value)
	switch field {
	case wfFieldName:
		name := workflowNormalizeName(value)
		if name == "" {
			return fmt.Errorf("name is required")
		}
		def.Name = name
		return nil
	case wfFieldDescription:
		if value == "" {
			return fmt.Errorf("description is required")
		}
		def.Description = value
		return nil
	}
	if taskIndex < 0 || taskIndex >= len(def.Tasks) {
		return fmt.Errorf("no task selected")
	}
	task := &def.Tasks[taskIndex]
	switch field {
	case wfFieldTaskID:
		id := workflowNormalizeName(value)
		if id == "" {
			return fmt.Errorf("task id is required")
		}
		old := task.ID
		for i, other := range def.Tasks {
			if i != taskIndex && other.ID == id {
				return fmt.Errorf("duplicate task id %q", id)
			}
		}
		task.ID = id
		if old != "" && old != id {
			for i := range def.Tasks {
				for j, dep := range def.Tasks[i].DependsOn {
					if dep == old {
						def.Tasks[i].DependsOn[j] = id
					}
				}
			}
		}
	case wfFieldTaskDescription:
		if value == "" {
			return fmt.Errorf("task description is required")
		}
		task.Description = value
	case wfFieldTaskPrompt:
		if value == "" {
			return fmt.Errorf("task prompt is required")
		}
		task.Prompt = value
	case wfFieldTaskDifficulty:
		task.Difficulty = strings.ToLower(value)
	case wfFieldTaskTools:
		task.AllowedTools = splitCSV(value)
	}
	return nil
}

func dependencyCandidates(def workflow.Definition, taskIndex int) []string {
	if taskIndex < 0 || taskIndex >= len(def.Tasks) {
		return nil
	}
	self := def.Tasks[taskIndex].ID
	out := make([]string, 0, len(def.Tasks))
	for i, task := range def.Tasks {
		if i == taskIndex {
			continue
		}
		id := task.ID
		if id == "" {
			id = fmt.Sprintf("task-%d", i+1)
		}
		if id == self {
			continue
		}
		out = append(out, id)
	}
	return out
}

func nextTaskID(tasks []workflow.TaskSpec) string {
	used := map[string]bool{}
	for i, task := range tasks {
		id := task.ID
		if id == "" {
			id = fmt.Sprintf("task-%d", i+1)
		}
		used[id] = true
	}
	for i := 1; i < 1000; i++ {
		id := fmt.Sprintf("step-%d", i)
		if !used[id] {
			return id
		}
	}
	return fmt.Sprintf("step-%d", len(tasks)+1)
}

func nextExecutionMode(current string) string {
	switch strings.ToLower(strings.TrimSpace(current)) {
	case "", "auto":
		return "serial"
	case "serial":
		return "parallel"
	case "parallel":
		return "auto"
	default:
		return "auto"
	}
}

func cloneWorkflowDefinition(def workflow.Definition) workflow.Definition {
	out := def
	out.Tasks = make([]workflow.TaskSpec, 0, len(def.Tasks))
	for _, task := range def.Tasks {
		out.Tasks = append(out.Tasks, workflow.TaskSpec{
			ID:           task.ID,
			Description:  task.Description,
			Prompt:       task.Prompt,
			AllowedTools: append([]string(nil), task.AllowedTools...),
			Difficulty:   task.Difficulty,
			Model:        task.Model,
			DependsOn:    append([]string(nil), task.DependsOn...),
		})
	}
	return out
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func removeStringValue(values []string, target string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			out = append(out, value)
		}
	}
	return out
}

func workflowNormalizeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimPrefix(name, "/")
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	return name
}
