package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderMessages(messages []ChatMessage, t Theme, showTimestamp bool, width int) string {
	var b strings.Builder
	// Leave a small gutter; no more box-border tax after message fences were removed.
	contentWidth := max(10, width-2)
	for _, msg := range messages {
		ts := renderTimestamp(msg, t, showTimestamp)
		switch msg.Role {
		case "welcome":
			renderWelcomeMessage(&b, msg, t, contentWidth)
		case "user":
			renderUserMessage(&b, msg, t, ts, contentWidth)
		case "assistant":
			renderAssistantMessage(&b, msg, t, ts, contentWidth)
		case "error":
			renderErrorMessage(&b, msg, t, ts, contentWidth)
		case "tool":
			renderToolStartMessage(&b, msg, t, ts, contentWidth)
		case "tool-done":
			renderToolDoneMessage(&b, msg, t, ts, contentWidth)
		case "agent":
			renderAgentMessage(&b, msg, t, ts, contentWidth)
		default:
			renderSystemMessage(&b, msg, t, ts, contentWidth)
		}
	}
	return b.String()
}

func renderTimestamp(msg ChatMessage, t Theme, showTimestamp bool) string {
	if !showTimestamp || msg.TimeStamp.IsZero() {
		return ""
	}
	return " " + t.Dim.Render(msg.TimeStamp.Format("15:04"))
}

func renderWelcomeMessage(b *strings.Builder, msg ChatMessage, t Theme, width int) {
	content := strings.TrimSpace(msg.Content)
	if content == "" {
		content = "Welcome to solcode"
	}

	// Keep the logo independent from the welcome copy so it stays centered even
	// when the terminal is resized or the copy changes length.
	logoWidth := max(1, width)
	center := func(value string) string {
		return lipgloss.NewStyle().Width(logoWidth).Align(lipgloss.Center).Render(value)
	}
	// No blank filler rows: the card previously spent 10 terminal lines on six
	// lines of content, which is expensive on short terminals.
	lines := []string{
		center(t.ClaudeStyle.Render("☀")),
		center(t.ClaudeStyle.Render("solcode")),
		center(t.Muted.Render(content)),
		center(t.Muted.Render("Ask a question, edit code, or run /help for commands.")),
	}
	body := strings.Join(lines, "\n")
	// width is the viewport width minus the border and horizontal padding. Use
	// it directly so the welcome card occupies the full available terminal row.
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(0, 2).
		Width(logoWidth).
		Render(body)
	b.WriteString(box)
	b.WriteString("\n")
}

func renderUserMessage(b *strings.Builder, msg ChatMessage, t Theme, ts string, width int) {
	content := strings.TrimSpace(msg.Content)
	if display := strings.TrimSpace(msg.DisplayContent); display != "" {
		content = display
	}
	if content == "" {
		return
	}
	// User turns keep a rounded card (no ❯/prompt marker) so they stay visually
	// distinct from assistant marker+connector output.
	body := strings.TrimRight(wrapIndent(content, max(10, width-4), ""), "\n")
	boxWidth := min(max(16, maxLineWidth(body)+4), max(16, width))
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Claude).
		Padding(0, 1).
		Width(boxWidth).
		Render(body)
	if ts != "" {
		b.WriteString(t.Dim.Render("You" + ts))
		b.WriteString("\n")
	}
	b.WriteString(box)
	b.WriteString("\n")
}

func renderAssistantMessage(b *strings.Builder, msg ChatMessage, t Theme, ts string, width int) {
	content := strings.TrimSpace(msg.Content)
	if content == "" {
		return
	}
	b.WriteString(t.Assistant.Render(AssistantMark) + ts + "\n")
	markdown := renderMarkdown(content, t, max(20, width-lipgloss.Width(Connector)))
	b.WriteString(leadBlock(markdown, t.Connector.Render(Connector)))
	b.WriteString("\n")
}

func renderErrorMessage(b *strings.Builder, msg ChatMessage, t Theme, ts string, width int) {
	content := strings.TrimSpace(msg.Content)
	if content == "" {
		return
	}
	b.WriteString(t.ErrorStyle.Render(ErrorMark) + ts + "\n")
	b.WriteString(leadBlock(wrapBody(content, width, Connector), t.Connector.Render(Connector)))
	b.WriteString("\n")
}

func renderSystemMessage(b *strings.Builder, msg ChatMessage, t Theme, ts string, width int) {
	content := strings.TrimSpace(msg.Content)
	if content == "" {
		return
	}
	b.WriteString(t.System.Render(SystemMark) + ts + "\n")
	b.WriteString(leadBlock(wrapBody(content, width, "  "), t.Muted.Render("  ")))
	b.WriteString("\n")
}

func renderToolStartMessage(b *strings.Builder, msg ChatMessage, t Theme, ts string, width int) {
	summary := toolInputSummary(msg.ToolName, msg.Content, width)
	title := AssistantMark + " " + msg.ToolName
	if summary != "" {
		title += " " + t.Dim.Render(summary)
	}
	b.WriteString(t.Tool.Render(title) + ts + "\n")
	body := "Running " + msg.ToolName + "…"
	if summary != "" {
		body += "\n" + summary
	}
	b.WriteString(leadBlock(wrapBody(body, width, Connector), t.Connector.Render(Connector)))
	b.WriteString("\n")
}

func renderToolDoneMessage(b *strings.Builder, msg ChatMessage, t Theme, ts string, width int) {
	title := AssistantMark + " " + msg.ToolName
	if msg.IsError {
		title += " failed"
		b.WriteString(t.ErrorStyle.Render(title) + ts + "\n")
	} else {
		b.WriteString(t.ToolDone.Render(title) + ts + "\n")
	}
	out := strings.TrimSpace(msg.Content)
	if out == "" {
		out = "(no output)"
	}
	collapsed := msg.Collapsed && !isFileMutationTool(msg.ToolName)
	if collapsed {
		lines := strings.Split(out, "\n")
		if len(lines) > 1 {
			out = lines[0] + fmt.Sprintf("\n… %d more lines (Ctrl+O to expand)", len(lines)-1)
		}
	}

	// Try inline diff rendering first
	diffContent := renderInlineDiff(out, t, width)
	if diffContent != "" {
		b.WriteString(leadBlock(diffContent, t.Connector.Render(Connector)))
		b.WriteString("\n")
		return
	}

	// Try syntax highlighting for file content from View/Read/Edit tools
	if isFileViewTool(msg.ToolName) && !msg.Collapsed {
		filePath := extractFilePath(msg.ToolName, out)
		if highlighted := renderCodeWithHighlight(out, filePath, t, width); highlighted != "" && highlighted != out {
			b.WriteString(leadBlock(highlighted, t.Connector.Render(Connector)))
			b.WriteString("\n")
			return
		}
	}

	b.WriteString(leadBlock(wrapBody(out, width, Connector), t.Connector.Render(Connector)))
	b.WriteString("\n")
}

// isFileViewTool returns true for tools that display file content.
func isFileViewTool(toolName string) bool {
	switch toolName {
	case "View", "Read", "mcp__filesystem__read-text-file", "mcp__filesystem__read-file",
		"mcp__filesystem__read-multiple-files", "Edit", "Write", "Patch":
		return true
	}
	return false
}

// extractFilePath tries to extract a file path from tool output or tool name context.
func extractFilePath(toolName string, content string) string {
	// Try to find a file path pattern in the first few lines
	lines := strings.Split(content, "\n")
	for i := 0; i < min(5, len(lines)); i++ {
		line := strings.TrimSpace(lines[i])
		// Common patterns: "File: path", "--- a/path", "+++ b/path"
		for _, prefix := range []string{"File: ", "Path: ", "--- a/", "+++ b/"} {
			if after, ok := strings.CutPrefix(line, prefix); ok {
				return strings.TrimSpace(after)
			}
		}
	}
	// Check first line for path-like strings
	if len(lines) > 0 {
		first := strings.TrimSpace(lines[0])
		if strings.Contains(first, "/") && !strings.Contains(first, " ") {
			return first
		}
	}
	return ""
}

func renderAgentMessage(b *strings.Builder, msg ChatMessage, t Theme, ts string, width int) {
	content := strings.TrimSpace(msg.Content)
	if content == "" {
		return
	}
	title := AssistantMark + " Agent"
	if msg.ToolName != "" {
		title += " " + msg.ToolName
	}
	if msg.IsError {
		b.WriteString(t.ErrorStyle.Render(title) + ts + "\n")
	} else {
		b.WriteString(t.AgentStyle.Render(title) + ts + "\n")
	}
	b.WriteString(leadBlock(wrapBody(content, width, Connector), t.Connector.Render(Connector)))
	b.WriteString("\n")
}

func indentBlock(text, prefix string) string {
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return ""
	}
	var b strings.Builder
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			b.WriteString("\n")
			continue
		}
		b.WriteString(prefix)
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// leadBlock marks only the first line with lead (e.g. the ⎿ connector) and
// aligns the remaining lines underneath it. Prefixing every line turns long
// output into a fenced wall and breaks nested markdown indentation.
func leadBlock(text, lead string) string {
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return ""
	}
	indent := strings.Repeat(" ", lipgloss.Width(lead))
	var b strings.Builder
	for i, line := range strings.Split(text, "\n") {
		if i == 0 {
			b.WriteString(lead)
			b.WriteString(line)
			b.WriteString("\n")
			continue
		}
		if strings.TrimSpace(line) == "" {
			b.WriteString("\n")
			continue
		}
		b.WriteString(indent)
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// wrapIndent wraps text to width, prefixing continuation lines with contPrefix.
func wrapIndent(text string, width int, contPrefix string) string {
	if width < 4 {
		width = 4
	}
	// contPrefix carries multi-byte glyphs and ANSI escapes, so its byte length
	// is far larger than the columns it occupies on screen.
	prefixWidth := lipgloss.Width(contPrefix)
	var b strings.Builder
	lines := strings.Split(text, "\n")
	for lineIndex, line := range lines {
		if lineIndex == 0 {
			wrapped := wrapLine(line, width)
			for i, w := range wrapped {
				if i == 0 {
					b.WriteString(w + "\n")
				} else {
					b.WriteString(contPrefix + w + "\n")
				}
			}
		} else {
			wrapped := wrapLine(line, width-prefixWidth)
			for _, w := range wrapped {
				b.WriteString(contPrefix + w + "\n")
			}
		}
	}
	return b.String()
}

// wrapBody wraps text into the column budget left over once lead occupies the
// gutter, without stamping lead onto each line. Pair it with leadBlock.
func wrapBody(text string, width int, lead string) string {
	body := max(10, width-lipgloss.Width(lead))
	var b strings.Builder
	for _, line := range strings.Split(text, "\n") {
		for _, wrapped := range wrapLine(line, body) {
			b.WriteString(wrapped + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func wrapLine(line string, width int) []string {
	if width < 1 {
		width = 1
	}
	if line == "" {
		return []string{""}
	}
	words := strings.Fields(line)
	if len(words) == 0 {
		return []string{""}
	}
	var out []string
	cur := ""
	for _, word := range words {
		// Hard-break CJK/long tokens that alone exceed the column budget.
		for lipgloss.Width(word) > width {
			chunk, rest := splitAtWidth(word, width)
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			if chunk != "" {
				out = append(out, chunk)
			}
			word = rest
		}
		if word == "" {
			continue
		}
		if cur == "" {
			cur = word
			continue
		}
		candidate := cur + " " + word
		if lipgloss.Width(candidate) <= width {
			cur = candidate
			continue
		}
		out = append(out, cur)
		cur = word
	}
	if cur != "" {
		out = append(out, cur)
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

// splitAtWidth cuts s so the head occupies at most width display columns.
func splitAtWidth(s string, width int) (head, tail string) {
	if width < 1 {
		width = 1
	}
	if lipgloss.Width(s) <= width {
		return s, ""
	}
	var b strings.Builder
	used := 0
	for i, r := range s {
		rw := lipgloss.Width(string(r))
		if used+rw > width {
			if b.Len() == 0 {
				// Pathological: a single rune wider than the budget — force it.
				return string(r), s[i+len(string(r)):]
			}
			return b.String(), s[i:]
		}
		b.WriteRune(r)
		used += rw
	}
	return b.String(), ""
}

func maxLineWidth(text string) int {
	width := 0
	for _, line := range strings.Split(text, "\n") {
		width = max(width, lipgloss.Width(line))
	}
	return width
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}

// truncateWidth trims by display columns rather than bytes, so CJK paths and
// other wide runes do not overflow the slot they were measured for.
func truncateWidth(value string, limit int) string {
	if limit <= 0 || lipgloss.Width(value) <= limit {
		return value
	}
	if limit <= 1 {
		return "…"
	}
	runes := []rune(value)
	var b strings.Builder
	used := 0
	for _, r := range runes {
		w := lipgloss.Width(string(r))
		if used+w > limit-1 {
			break
		}
		b.WriteRune(r)
		used += w
	}
	return b.String() + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
