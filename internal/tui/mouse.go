package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func settingsRows() [][]string {
	return [][]string{{"claude", "codex", "grok", "opencode", "kilo"}, {"dark", "light"}, {"blue", "green", "purple", "orange", "rose"}, {"cyan", "ocean", "sunset", "grape", "forest"}, {"Done"}}
}
func (m *Model) mouse(msg tea.MouseMsg) tea.Cmd {
	if m.screen == "create" {
		if m.wizard != nil {
			return m.wizardMouse(msg)
		}
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		return cmd
	}
	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
		if msg.X >= max(20, m.width/3) || m.screen == "log" || m.screen == "create-preview" {
			var cmd tea.Cmd
			m.preview, cmd = m.preview.Update(msg)
			return cmd
		}
		delta := 1
		if msg.Button == tea.MouseButtonWheelUp {
			delta = -1
		}
		m.cursor = max(0, min(m.count()-1, m.cursor+delta))
		m.updatePreview()
		return nil
	}
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return nil
	}
	if m.screen == "settings" {
		top := 3
		if m.firstRun {
			top += 2
		}
		row := (msg.Y - top) / 2
		if msg.Y < top || (msg.Y-top)%2 != 0 || row >= len(settingsRows()) {
			return nil
		}
		column := msg.X / 16
		if column >= len(settingsRows()[row]) {
			return nil
		}
		m.settingsRow, m.settingsColumn = row, column
		return m.settingsKey("enter")
	}
	// Resolve the lower input rows from rendered terminal cells, so a resize
	// or wrapped detail panel cannot shift clicks onto the wrong control.
	lines := strings.Split(ansi.Strip(m.View()), "\n")
	if m.screen == "vibe" {
		for y, line := range lines {
			if strings.HasPrefix(line, "Mode:") {
				if msg.Y == y {
					position := 6
					for i, mode := range modes {
						width := len(mode) + 3
						if msg.X >= position && msg.X < position+width {
							m.mode = i
							return nil
						}
						position += width
					}
				}
				if msg.Y == y+1 {
					m.focus = "question"
					m.filter.Blur()
					return m.question.Focus()
				}
			}
		}
	}
	top := 3
	if m.screen == "vibe" || m.screen == "history" {
		if msg.Y == top {
			m.focus = "filter"
			m.question.Blur()
			return m.filter.Focus()
		}
		top++
	}
	if msg.X >= max(20, m.width/3) || msg.Y < top {
		return nil
	}
	visible := max(3, m.height-12)
	start := max(0, m.cursor-visible+1)
	index := start + msg.Y - top
	if index < 0 || index >= m.count() || msg.Y-top >= visible {
		return nil
	}
	wasSelected := index == m.cursor
	m.cursor = index
	m.updatePreview()
	if wasSelected && (m.screen == "hub" || m.screen == "vibe") {
		return m.activate()
	}
	return nil
}
