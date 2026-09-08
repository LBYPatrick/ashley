package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func settingsRows() [][]string {
	return [][]string{{"claude", "codex", "grok", "opencode", "kilo"}, {"dark", "light"}, {"blue", "green", "purple", "orange", "rose"}, {"cyan", "ocean", "sunset", "grape", "forest"}, {"Done"}}
}
func (m *Model) mouse(msg tea.MouseMsg) tea.Cmd {
	if m.paletteOpen {
		r := m.paletteBounds()
		visible := max(1, r.h-7)
		start := max(0, m.paletteCursor-visible+1)
		if msg.Button == tea.MouseButtonWheelDown {
			m.paletteCursor = min(max(0, len(m.paletteItems())-1), m.paletteCursor+1)
		}
		if msg.Button == tea.MouseButtonWheelUp {
			m.paletteCursor = max(0, m.paletteCursor-1)
		}
		if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress && msg.X >= r.x+2 && msg.X < r.x+r.w-2 && msg.Y >= r.y+5 && msg.Y < r.y+5+visible {
			index := start + msg.Y - r.y - 5
			if index < len(m.paletteItems()) {
				m.paletteCursor = index
				return m.paletteKey(tea.KeyMsg{Type: tea.KeyEnter})
			}
		}
		return nil
	}
	wheel := msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown
	delta := 1
	if msg.Button == tea.MouseButtonWheelUp {
		delta = -1
	}
	if m.screen == "settings" {
		if wheel {
			m.screenScroll = max(0, min(max(0, m.settingsContentHeight()-m.height), m.screenScroll+delta*3))
			return nil
		}
		if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress && msg.Y > 0 && msg.Y < m.height-1 {
			for _, control := range m.settingsControls() {
				if control.contains(msg.X, msg.Y+m.screenScroll) {
					m.settingsRow, m.settingsColumn = control.row, control.column
					return m.settingsKey("enter")
				}
			}
		}
		return nil
	}
	if m.screen == "create" {
		return m.creatorMouse(msg)
	}
	if m.screen == "sync" {
		if wheel {
			m.screenScroll = max(0, min(m.operationMaxScroll(), m.screenScroll+delta*3))
		}
		return nil
	}
	if m.screen == "stats" && wheel {
		m.screenScroll = max(0, min(m.statsMaxScroll(), m.screenScroll+delta*3))
		return nil
	}
	l := m.layout()
	if wheel {
		_, log, _ := m.sessionPanels()
		if m.screen == "sessions" && log.contains(msg.X, msg.Y) {
			m.logOffset = max(0, min(m.maxLogOffset(), m.logOffset+delta*3))
			return nil
		}
		if l.right.contains(msg.X, msg.Y) || m.screen == "log" || m.screen == "help" || m.screen == "create-preview" {
			var cmd tea.Cmd
			m.preview, cmd = m.preview.Update(msg)
			return cmd
		}
		m.cursor = max(0, min(m.count()-1, m.cursor+delta))
		m.updatePreview()
		return nil
	}
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return nil
	}
	if (m.screen == "vibe" || m.screen == "history") && l.input.contains(msg.X, msg.Y) {
		if m.screen == "vibe" {
			m.focus = "question"
			m.filter.Blur()
			return m.question.Focus()
		}
		m.focus = "filter"
		return m.filter.Focus()
	}
	if m.screen == "vibe" && l.mode.contains(msg.X, msg.Y) {
		if l.mode.w < 70 {
			m.mode = (m.mode + 1) % 4
			return nil
		}
		x := l.mode.x + 9
		for index, width := range []int{11, 8, 9, 8} {
			if msg.X >= x && msg.X < x+width {
				m.mode = index
				return nil
			}
			x += width
		}
	}
	if l.list.contains(msg.X, msg.Y) {
		index := max(0, m.cursor-l.list.h+1) + msg.Y - l.list.y
		if index >= m.count() {
			return nil
		}
		selected := index == m.cursor
		m.cursor = index
		m.focus = ""
		m.filter.Blur()
		m.question.Blur()
		m.updatePreview()
		if selected && (m.screen == "hub" || m.screen == "vibe") {
			return m.activate()
		}
	}
	return nil
}
