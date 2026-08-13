package unit_tests

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/solosw/solcode/internal/tui"
)

func TestTUIModelLongPathPasteStaysWithinComposerWidth(t *testing.T) {
	const width = 80
	model := tui.New(func(string) (tea.Cmd, func()) { return nil, nil })
	updated, _ := model.Update(tea.WindowSizeMsg{Width: width, Height: 20})
	model = updated.(tui.Model)

	path := `C:\\software\\projects\\solcode\\internal\\very-long-directory-name\\another-very-long-directory-name\\screen-shot.png`
	updated, _ = model.Update(tea.PasteMsg{Content: path})
	model = updated.(tui.Model)

	view := model.View().Content
	for _, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("composer line width = %d, exceeds terminal width %d: %q", got, width, line)
		}
	}
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "╭") || strings.Contains(line, "╰") {
			if got := lipgloss.Width(line); got != width {
				t.Fatalf("composer border width = %d, want terminal width %d: %q", got, width, line)
			}
		}
	}
}

func TestTUIModelLongPathKeyInputStaysWithinComposerWidth(t *testing.T) {
	const width = 80
	model := tui.New(func(string) (tea.Cmd, func()) { return nil, nil })
	updated, _ := model.Update(tea.WindowSizeMsg{Width: width, Height: 20})
	model = updated.(tui.Model)

	path := `C:\\software\\projects\\solcode\\internal\\very-long-directory-name\\another-very-long-directory-name\\screen-shot.png`
	for _, r := range path {
		updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: string(r)}))
		model = updated.(tui.Model)
	}

	for _, line := range strings.Split(model.View().Content, "\n") {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("key input line width = %d, exceeds terminal width %d: %q", got, width, line)
		}
	}
}

func TestTUIModelLongPathInputDoesNotHorizontallyScrollTranscript(t *testing.T) {
	model := tui.New(func(string) (tea.Cmd, func()) { return nil, nil })
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	model = updated.(tui.Model)

	path := `C:\\software\\projects\\solcode\\internal\\long-file-name.dll`
	for _, r := range path {
		updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: string(r)}))
		model = updated.(tui.Model)
	}

	if !strings.Contains(model.View().Content, path) {
		t.Fatalf("expected pasted path to remain in input view")
	}
}

func TestTUIModelFoldsOnlyLargeExplicitPaste(t *testing.T) {
	model := tui.New(nil)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	model = updated.(tui.Model)

	updated, _ = model.Update(tea.PasteMsg{Content: "one\ntwo"})
	model = updated.(tui.Model)
	view := model.View().Content
	if !strings.Contains(view, "one") || !strings.Contains(view, "two") || strings.Contains(view, "Pasted text #") {
		t.Fatalf("expected short explicit paste to remain inline: %s", view)
	}

	largePaste := "a\nb\nc\nd\ne"
	updated, _ = model.Update(tea.PasteMsg{Content: largePaste})
	model = updated.(tui.Model)
	view = model.View().Content
	if !strings.Contains(view, "Pasted text #1 · 5 lines") {
		t.Fatalf("expected large explicit paste to fold: %s", view)
	}
}

func TestTUIModelExpandsMultipleFoldedPastesInOrder(t *testing.T) {
	var submitted string
	model := tui.New(func(prompt string) (tea.Cmd, func()) {
		submitted = prompt
		return nil, nil
	})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	model = updated.(tui.Model)

	first := "first-1\nfirst-2\nfirst-3\nfirst-4\nfirst-5"
	second := "second-1\nsecond-2\nsecond-3\nsecond-4\nsecond-5"
	for _, paste := range []string{first, second} {
		updated, _ = model.Update(tea.PasteMsg{Content: paste})
		model = updated.(tui.Model)
	}
	time.Sleep(200 * time.Millisecond)
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(tui.Model)

	firstMarker := "--- Begin [Pasted text #1 · 5 lines] ---"
	secondMarker := "--- Begin [Pasted text #2 · 5 lines] ---"
	if !strings.Contains(submitted, firstMarker) || !strings.Contains(submitted, secondMarker) || !strings.Contains(submitted, first) || !strings.Contains(submitted, second) {
		t.Fatalf("expected both folded pastes to expand, got %q", submitted)
	}
	if strings.Index(submitted, firstMarker) > strings.Index(submitted, secondMarker) {
		t.Fatalf("expected folded pastes to retain input order, got %q", submitted)
	}
}

func TestTUIModelDeletedFoldedPasteIsNotSubmitted(t *testing.T) {
	var submitted string
	model := tui.New(func(prompt string) (tea.Cmd, func()) {
		submitted = prompt
		return nil, nil
	})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	model = updated.(tui.Model)

	paste := "secret-1\nsecret-2\nsecret-3\nsecret-4\nsecret-5"
	updated, _ = model.Update(tea.PasteMsg{Content: paste})
	model = updated.(tui.Model)
	for range "[Pasted text #1 · 5 lines] " {
		updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace}))
		model = updated.(tui.Model)
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "keep"}))
	model = updated.(tui.Model)
	time.Sleep(200 * time.Millisecond)
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(tui.Model)

	if submitted != "keep" {
		t.Fatalf("expected deleted folded paste to be omitted, got %q", submitted)
	}
}

func TestTUIModelLongPasteKeepsComposerVisible(t *testing.T) {
	model := tui.New(nil)
	const height = 20
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: height})
	model = updated.(tui.Model)

	updated, _ = model.Update(tea.PasteMsg{Content: strings.Repeat("long pasted line\n", 356)})
	model = updated.(tui.Model)
	view := model.View().Content

	if got := strings.Count(view, "\n") + 1; got > height {
		t.Fatalf("view height = %d, terminal height = %d; long paste pushed composer off screen", got, height)
	}
	if !strings.Contains(view, "╭") || !strings.Contains(view, "ctx ") {
		t.Fatalf("expected fixed composer and usage bar after long paste: %s", view)
	}
}

func TestTUIModelShortTerminalHidesAutocompleteBeforeComposer(t *testing.T) {
	model := tui.New(nil)
	const height = 8
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: height})
	model = updated.(tui.Model)
	model, _ = setInputValue(model, "/")
	view := model.View().Content

	if got := strings.Count(view, "\n") + 1; got > height {
		t.Fatalf("view height = %d, terminal height = %d; autocomplete pushed composer off screen", got, height)
	}
	if !strings.Contains(view, "╭") || !strings.Contains(view, "ctx ") {
		t.Fatalf("expected fixed composer and usage bar on a short terminal: %s", view)
	}
	if strings.Contains(view, "Commands:") {
		t.Fatalf("expected autocomplete to hide when it cannot fit: %s", view)
	}
}

func TestTUIModelRestoredFoldedPasteKeepsComposerVisible(t *testing.T) {
	model := tui.New(nil)
	const height = 20
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: height})
	model = updated.(tui.Model)

	label := "[Pasted text #1 · 356 lines]"
	body := strings.Repeat("long pasted line\n", 356)
	model.ReplaceMessages([]tui.ChatMessage{{
		Role:           "user",
		Content:        label + "\n\n--- Begin " + label + " ---\n" + body + "--- End " + label + " ---",
		DisplayContent: label,
	}})
	view := model.View().Content

	if got := strings.Count(view, "\n") + 1; got > height {
		t.Fatalf("view height = %d, terminal height = %d; restored paste pushed composer off screen", got, height)
	}
	if !strings.Contains(view, label) || strings.Contains(view, "long pasted line") {
		t.Fatalf("expected restored paste to render as its compact label: %s", view)
	}
	if !strings.Contains(view, "╭") || !strings.Contains(view, "ctx ") {
		t.Fatalf("expected fixed composer and usage bar after restoring a folded paste: %s", view)
	}
}
