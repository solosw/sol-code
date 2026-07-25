package tui

import "github.com/charmbracelet/lipgloss"

// Theme holds the color palette and derived lipgloss styles for the TUI.
// Two built-in palettes are provided: Dark and Light, mimicking Claude Code.
type Theme struct {
	Name string

	Claude             lipgloss.Color
	ClaudeShimmer      lipgloss.Color
	Text               lipgloss.Color
	Inactive           lipgloss.Color
	Subtle             lipgloss.Color
	Suggestion         lipgloss.Color
	Success            lipgloss.Color
	Error              lipgloss.Color
	Warning            lipgloss.Color
	Permission         lipgloss.Color
	PromptBorder       lipgloss.Color
	Background         lipgloss.Color
	BackgroundOverride string
	StatusBG           lipgloss.Color
	StatusFG           lipgloss.Color

	// Semantic slots so tool/agent/system output can be told apart at a glance
	// instead of all reusing the accent color.
	Border       lipgloss.Color
	BorderSubtle lipgloss.Color
	PanelBG      lipgloss.Color
	TextMuted    lipgloss.Color
	Agent        lipgloss.Color
	ToolRunning  lipgloss.Color

	// Derived styles
	User         lipgloss.Style
	Assistant    lipgloss.Style
	System       lipgloss.Style
	ErrorStyle   lipgloss.Style
	Tool         lipgloss.Style
	ToolDone     lipgloss.Style
	ToolResult   lipgloss.Style
	Connector    lipgloss.Style
	Dim          lipgloss.Style
	Muted        lipgloss.Style
	AgentStyle   lipgloss.Style
	PanelTitle   lipgloss.Style
	PanelBar     lipgloss.Style
	ClaudeStyle  lipgloss.Style
	Status       lipgloss.Style
	Prompt       lipgloss.Style
	PermBorder   lipgloss.Style
	DialogBorder lipgloss.Style
	PermTitle    lipgloss.Style
	PermHint     lipgloss.Style
	DiffAdd      lipgloss.Style
	DiffDel      lipgloss.Style
	DiffCtx      lipgloss.Style
	DiffHdr      lipgloss.Style
}

var Dark = buildTheme("dark", themePalette{
	claude:        "#d77757",
	claudeShimmer: "#e8a98a",
	text:          "#ffffff",
	inactive:      "#999999",
	subtle:        "#505050",
	suggestion:    "#6495ed",
	success:       "#4eba65",
	err:           "#ff6b80",
	warning:       "#ffc107",
	permission:    "#b1b9f9",
	promptBorder:  "#888888",
	background:    "#000000",
	statusBG:      "#1c1c1c",
	statusFG:      "#bcbcbc",
	border:        "#4a4a4a",
	borderSubtle:  "#2e2e2e",
	panelBG:       "#141414",
	textMuted:     "#7a7a7a",
	agent:         "#9d7cd8",
	toolRunning:   "#57b6c2",
})

var Light = buildTheme("light", themePalette{
	claude:        "#b85c3a",
	claudeShimmer: "#d98a6a",
	text:          "#222222",
	inactive:      "#666666",
	subtle:        "#aaaaaa",
	suggestion:    "#3b6fd4",
	success:       "#2f8a46",
	err:           "#d23a52",
	warning:       "#c98a00",
	permission:    "#6f78c8",
	promptBorder:  "#bbbbbb",
	background:    "#fafafa",
	statusBG:      "#ebebeb",
	statusFG:      "#4a4a4a",
	border:        "#c3c3c3",
	borderSubtle:  "#dcdcdc",
	panelBG:       "#f2f2f2",
	textMuted:     "#8a8a8a",
	agent:         "#7a4fbf",
	toolRunning:   "#2f7f8c",
})

type themePalette struct {
	claude, claudeShimmer, text, inactive, subtle, suggestion string
	success, err, warning, permission, promptBorder           string
	background, statusBG, statusFG                            string
	border, borderSubtle, panelBG, textMuted                  string
	agent, toolRunning                                        string
}

func buildTheme(name string, p themePalette) Theme {
	t := Theme{
		Name:          name,
		Claude:        lipgloss.Color(p.claude),
		ClaudeShimmer: lipgloss.Color(p.claudeShimmer),
		Text:          lipgloss.Color(p.text),
		Inactive:      lipgloss.Color(p.inactive),
		Subtle:        lipgloss.Color(p.subtle),
		Suggestion:    lipgloss.Color(p.suggestion),
		Success:       lipgloss.Color(p.success),
		Error:         lipgloss.Color(p.err),
		Warning:       lipgloss.Color(p.warning),
		Permission:    lipgloss.Color(p.permission),
		PromptBorder:  lipgloss.Color(p.promptBorder),
		Background:    lipgloss.Color(p.background),
		StatusBG:      lipgloss.Color(p.statusBG),
		StatusFG:      lipgloss.Color(p.statusFG),
		Border:        lipgloss.Color(p.border),
		BorderSubtle:  lipgloss.Color(p.borderSubtle),
		PanelBG:       lipgloss.Color(p.panelBG),
		TextMuted:     lipgloss.Color(p.textMuted),
		Agent:         lipgloss.Color(p.agent),
		ToolRunning:   lipgloss.Color(p.toolRunning),
	}
	t.User = lipgloss.NewStyle().Foreground(t.Claude).Bold(true)
	t.Assistant = lipgloss.NewStyle().Foreground(t.Claude)
	t.System = lipgloss.NewStyle().Foreground(t.Suggestion).Italic(true)
	t.ErrorStyle = lipgloss.NewStyle().Foreground(t.Error).Bold(true)
	t.Tool = lipgloss.NewStyle().Foreground(t.ToolRunning)
	t.ToolDone = lipgloss.NewStyle().Foreground(t.Success)
	t.ToolResult = lipgloss.NewStyle().Faint(true)
	t.Connector = lipgloss.NewStyle().Foreground(t.BorderSubtle)
	t.Dim = lipgloss.NewStyle().Foreground(t.Inactive)
	t.Muted = lipgloss.NewStyle().Foreground(t.TextMuted)
	t.AgentStyle = lipgloss.NewStyle().Foreground(t.Agent)
	t.PanelTitle = lipgloss.NewStyle().Foreground(t.TextMuted).Bold(true)
	t.PanelBar = lipgloss.NewStyle().Foreground(t.Border)
	t.ClaudeStyle = lipgloss.NewStyle().Foreground(t.Claude).Bold(true)
	t.Status = lipgloss.NewStyle().Background(t.StatusBG).Foreground(t.StatusFG).Padding(0, 1)
	t.Prompt = lipgloss.NewStyle().Foreground(t.Claude).Bold(true)
	t.PermBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Permission).
		BorderBottom(false).BorderLeft(false).BorderRight(false).
		MarginTop(1)
	// Rounded everywhere so message cards, dialogs and the prompt speak one
	// border language instead of mixing rounded and square frames on screen.
	t.DialogBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Permission).
		Padding(0, 1).
		MarginTop(0)
	t.PermTitle = lipgloss.NewStyle().Foreground(t.Permission).Bold(true)
	t.PermHint = lipgloss.NewStyle().Foreground(t.Inactive)
	t.DiffAdd = lipgloss.NewStyle().Foreground(t.Success)
	t.DiffDel = lipgloss.NewStyle().Foreground(t.Error)
	t.DiffCtx = lipgloss.NewStyle().Foreground(t.Inactive)
	t.DiffHdr = lipgloss.NewStyle().Foreground(t.Suggestion).Bold(true)
	return t
}

// ProgressColor grades a usage meter from calm to alarming so a nearly full
// context window reads as a warning without needing extra copy.
func (t Theme) ProgressColor(used, limit int64) lipgloss.Color {
	if limit <= 0 || used <= 0 {
		return t.Success
	}
	switch ratio := float64(used) / float64(limit); {
	case ratio >= 0.9:
		return t.Error
	case ratio >= 0.7:
		return t.Warning
	default:
		return t.Success
	}
}

func ThemeByName(name string) Theme {
	if name == "light" {
		return Light
	}
	return Dark
}

// WithBackground returns a copy of theme with a custom TUI background. An empty
// value preserves the palette default. The color value is passed to lipgloss, so
// it may be a hexadecimal color or an ANSI color index.
func (t Theme) WithBackground(background string) Theme {
	if background == "" {
		return t
	}
	t.Background = lipgloss.Color(background)
	t.BackgroundOverride = background
	// Chrome that paints its own fill (status bar, panels) must follow the new
	// background, otherwise it shows up as an unrelated band across the screen.
	if base, ok := parseHexColor(background); ok {
		accent, accentOK := parseHexColor(string(t.Claude))
		if !accentOK {
			accent = base
		}
		t.StatusBG = lipgloss.Color(interpolateHex(base, accent, 0.12))
		t.PanelBG = lipgloss.Color(interpolateHex(base, accent, 0.07))
		t.Status = t.Status.Background(t.StatusBG)
	}
	return t
}
