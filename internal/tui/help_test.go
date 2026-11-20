package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestShortcutHelpSearchAndReturn(t *testing.T) {
	m := newModel(t)
	m.open("sessions")
	key(m, "/")
	key(m, "existing search")
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	key(m, "Keys")
	key(m, "enter")
	if view := ansi.Strip(m.View()); !strings.Contains(view, "Current screen") || !strings.Contains(view, "Search shortcuts…") {
		t.Fatal("shortcut reference is missing context or search")
	}
	key(m, "/")
	key(m, "copy prompt")
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "Skill browser") || !strings.Contains(view, "Copy prompt") || strings.Contains(view, "Navigation") {
		t.Fatal("search did not filter the shortcut groups")
	}
	key(m, "esc")
	if m.screen != "sessions" || m.filter.Value() != "existing search" || m.focus != "filter" {
		t.Fatal("leaving help changed the original search")
	}
}

func TestShortcutHelpLayoutAndLocalizedSearch(t *testing.T) {
	m := newModel(t)
	m.language = "ja"
	m.openShortcutHelp()
	for _, size := range [][2]int{{20, 8}, {48, 12}, {100, 30}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		assertFrame(t, m.View(), size[0], size[1])
	}
	exportRegressionView(t, "shortcut-reference-ja", m.View())
	key(m, "/")
	key(m, "名前を変更")
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "名前を変更") || !strings.Contains(view, " N ") {
		t.Fatal("localized search lost the action or its key")
	}
	m.help.search.SetValue("no matching shortcut")
	if !strings.Contains(ansi.Strip(m.View()), m.tr("No matching shortcuts")) {
		t.Fatal("empty search has no feedback")
	}
}
