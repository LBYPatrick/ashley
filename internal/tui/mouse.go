package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func settingsRows() [][]string {
	return [][]string{{"claude", "codex", "grok", "opencode", "kilo"}, {"auto", "dark", "light"}, {"blue", "green", "purple", "orange", "rose"}, {"cyan", "ocean", "sunset", "grape", "forest"}, {"Done"}}
}
func (m *Model) mouse(msg tea.MouseMsg) tea.Cmd {
	if m.rename != nil {
		return nil
	}
	if m.confirm != nil {
		if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
			r := m.confirmBounds()
			if msg.Y == r.y+r.h-2 {
				for i := 0; i < 2; i++ {
					if msg.X >= r.x+3+i*13 && msg.X < r.x+12+i*13 {
						m.confirm.proceed = i == 1
						return m.confirmKey("enter")
					}
				}
			}
		}
		return nil
	}
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
	if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
		if msg.Y == 0 && msg.X >= max(2, m.width-15) {
			m.paletteOpen = true
			m.paletteQuery = ""
			m.paletteCursor = 0
			return nil
		}
		for _, c := range m.navControls() {
			if c.contains(msg.X, msg.Y) {
				m.open(c.key)
				return nil
			}
		}
	}
	if m.screen == "compose" {
		if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
			r := m.composeBounds()
			if msg.X < r.x || msg.X >= r.x+r.w {
				return nil
			}
			in := m.composerInput()
			if in.contains(msg.X, msg.Y) {
				m.composeFocus = 0
				return m.composer.Focus()
			}
			if msg.Y == in.y-2 {
				m.open("vibe")
				return nil
			}
			y := in.y + in.h + 1
			if msg.Y == y {
				m.composeFocus = 1
				m.composer.Blur()
				m.cycleAgent()
			}
			if msg.Y == y+1 {
				m.composeFocus = 2
				m.composer.Blur()
				m.mode = (m.mode + 1) % len(modes)
			}
			if msg.Y == y+5 {
				return m.launchDraft()
			}
		}
		return nil
	}
	if m.screen == "hub" {
		if msg.Button == tea.MouseButtonWheelDown {
			m.cursor = min(m.count()-1, m.cursor+1)
		}
		if msg.Button == tea.MouseButtonWheelUp {
			m.cursor = max(0, m.cursor-1)
		}
		if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
			for _, c := range m.homeControls() {
				if c.contains(msg.X, msg.Y) {
					m.cursor = c.row
					return m.activate()
				}
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
	if !m.compactDetail && (m.screen == "sessions" || m.screen == "history") && msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress && msg.Y == l.left.y {
		if msg.X >= l.left.x && msg.X < l.left.x+10 {
			m.open("sessions")
			return nil
		}
		if msg.X >= l.left.x+12 && msg.X < l.left.x+22 {
			m.open("history")
			return nil
		}
	}
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
	if m.screen == "skills.sh" && m.skillsHasInput() && l.input.contains(msg.X, msg.Y) {
		m.focus = "skills-input"
		return m.question.Focus()
	}
	if m.compactDetail {
		return nil
	}
	if (m.screen == "vibe" || m.screen == "history") && l.input.contains(msg.X, msg.Y) {
		m.focus = "filter"
		return m.filter.Focus()
	}
	if l.list.contains(msg.X, msg.Y) {
		index := max(0, m.cursor-m.visibleRows()+1) + (msg.Y-l.list.y)/m.rowHeight()
		if index >= m.count() {
			return nil
		}
		selected := index == m.cursor
		m.cursor = index
		m.focus = ""
		m.filter.Blur()
		m.question.Blur()
		m.updatePreview()
		if selected && (m.screen == "hub" || m.screen == "vibe" || m.screen == "skills.sh") {
			return m.activate()
		}
	}
	return nil
}
