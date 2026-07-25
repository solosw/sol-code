package tui

import (
	"os"
	"strings"
)

// Claude Code-style message markers and connectors.
const (
	UserMark      = "❯"
	AssistantMark = "●"
	SystemMark    = "▶"
	ErrorMark     = "⚠"
	Connector     = "  ⎿  "
	Continuation  = "  │  "
	PromptPrefix  = "> "
)

// Spinner frames (Claude style, forward then reverse).
var SpinnerFrames = []string{"·", "✢", "*", "✶", "✻", "✽", "✻", "✶", "*", "✢"}

// Glyphs collects the drawing characters used by the chrome (scrollbars, meters,
// panels). Terminals that cannot render box drawing characters fall back to a
// pure ASCII set so the layout degrades instead of turning into mojibake.
type Glyphs struct {
	ScrollTrack string
	ScrollThumb string
	BarFilled   string
	BarEmpty    string
	TodoPending string
	TodoActive  string
	TodoDone    string
	PanelBar    string
	Separator   string
	Idle        string
}

var unicodeGlyphs = Glyphs{
	ScrollTrack: "╎",
	ScrollThumb: "▐",
	BarFilled:   "▰",
	BarEmpty:    "▱",
	TodoPending: "○",
	TodoActive:  "◐",
	TodoDone:    "●",
	PanelBar:    "│",
	Separator:   "·",
	Idle:        "·",
}

var asciiGlyphs = Glyphs{
	ScrollTrack: ":",
	ScrollThumb: "#",
	BarFilled:   "=",
	BarEmpty:    "-",
	TodoPending: "[ ]",
	TodoActive:  "[>]",
	TodoDone:    "[x]",
	PanelBar:    "|",
	Separator:   "-",
	Idle:        ".",
}

// glyphs is resolved once at startup; terminal capability does not change while
// the program runs.
var glyphs = pickGlyphs()

// ActiveGlyphs returns the glyph set in use for the current terminal.
func ActiveGlyphs() Glyphs { return glyphs }

// asciiOnlyTerminal reports whether the TUI should avoid box drawing glyphs.
// Detection is opt-in via SOLCODE_ASCII plus a non-UTF-8 locale check, because
// guessing from TERM alone produces false positives on modern terminals.
func asciiOnlyTerminal() bool {
	for _, key := range []string{"SOLCODE_ASCII", "SOLCODE_TUI_ASCII"} {
		switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
		case "1", "true", "yes", "on":
			return true
		}
	}
	for _, key := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return !strings.Contains(strings.ToUpper(value), "UTF")
		}
	}
	return false
}

func pickGlyphs() Glyphs {
	if asciiOnlyTerminal() {
		return asciiGlyphs
	}
	return unicodeGlyphs
}
