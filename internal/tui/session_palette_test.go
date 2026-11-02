package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSessionActionsPaletteHasOnlySessionActions(t *testing.T) {
	m := activitySearchModel(t)
	m.open("sessions")
	m.filter.SetValue("/workspace")
	m.directoryOnly = true
	m.refresh()
	m.cursor = 1
	m.openSessionActions()
	items := m.paletteItems()
	if len(items) != 2 || items[0][0] != "Kill all sessions" || items[1][0] != "Clean finished sessions" {
		t.Fatal("Actions palette includes unrelated commands", items)
	}
	key(m, "clean")
	if items := m.paletteItems(); len(items) != 1 || items[0][0] != "Clean finished sessions" {
		t.Fatal("Actions palette cannot narrow its commands", items)
	}
	key(m, "esc")
	if m.paletteOpen || m.paletteSessionActions || m.filter.Value() != "/workspace" || !m.directoryOnly || m.cursor != 1 {
		t.Fatal("closing Actions changed browser state or retained the palette scope")
	}
	m.openSessionActions()
	key(m, "clean")
	key(m, "enter")
	if m.paletteOpen || m.paletteSessionActions || m.confirm == nil || m.confirm.key != "cleanup" {
		t.Fatal("Actions did not open its confirmation and reset its palette scope")
	}
	if m.filter.Value() != "/workspace" || !m.directoryOnly || m.cursor != 1 {
		t.Fatal("accepting an Actions command changed browser state")
	}
	m.confirm = nil
	m.openSessionActions()
	key(m, "unmatched")
	key(m, "enter")
	if m.paletteOpen || m.paletteSessionActions || m.confirm != nil {
		t.Fatal("Enter with no action retained the palette scope or started an action")
	}
}

func TestGlobalPaletteResetsSessionActionsScope(t *testing.T) {
	for _, route := range []string{"keyboard", "header", "navigation"} {
		t.Run(route, func(t *testing.T) {
			m := newModel(t)
			m.open("sessions")
			m.openSessionActions()
			switch route {
			case "keyboard":
				m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
				m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
			case "header":
				m.Update(tea.MouseMsg{X: m.width - 3, Y: 0, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
			case "navigation":
				m.Update(tabShortcut(1))
				m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
			}
			if !m.paletteOpen || m.paletteSessionActions {
				t.Fatal("global command access retained the Actions scope")
			}
			m.paletteQuery = "version"
			if items := m.paletteItems(); len(items) != 1 || items[0][0] != "About Ashley" {
				t.Fatal("global palette cannot find About", items)
			}
		})
	}
}
