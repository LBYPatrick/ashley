package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"strings"
)

func settingsRows() [][]string {
	return [][]string{{"claude", "codex", "grok", "opencode", "kilo"}, {"dark", "light"}, {"blue", "green", "purple", "orange", "rose"}, {"cyan", "ocean", "sunset", "grape", "forest"}, {"Done"}}
}
func (m *Model) mouse(msg tea.MouseMsg) tea.Cmd {
	if m.paletteOpen {
		if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress && msg.Y >= 8 {
			index := (msg.Y - 8) / 2
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
			m.screenScroll = max(0, min(max(0, 38-m.height), m.screenScroll+delta*3))
			return nil
		}
		if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
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
		if m.wizard != nil && m.wizard.stage == 0 {
			if wheel {
				m.screenScroll = max(0, min(max(0, 38-m.height), m.screenScroll+delta*3))
				return nil
			}
			if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
				y := msg.Y + m.screenScroll
				for index, top := range []int{9, 14, 22, 29} {
					if y >= top && y < top+3 {
						m.wizard.collect()
						m.wizard.field = index
						m.wizard.loadField()
						m.keepCreatorVisible()
						return nil
					}
				}
				if y == 34 {
					if msg.X < 14 {
						return m.wizardKey(tea.KeyMsg{Type: tea.KeyCtrlN})
					}
					return m.wizardKey(tea.KeyMsg{Type: tea.KeyCtrlS})
				}
			}
			return nil
		}
		if m.wizard != nil {
			return m.wizardMouse(msg)
		}
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		return cmd
	}
	if m.screen == "stats" && wheel {
		m.screenScroll = max(0, m.screenScroll+delta*3)
		return nil
	}
	l := m.layout()
	if wheel {
		if m.screen == "sessions" && l.right.contains(msg.X, msg.Y) && msg.Y >= l.detail.y+min(len(strings.Split(ansi.Wrap(m.detailText(), l.detail.w, ""), "\n")), max(3, l.detail.h-7))+1 {
			m.logOffset = max(0, min(max(0, len(strings.Split(m.sessionLog, "\n"))-3), m.logOffset+delta*3))
			return nil
		}
		if l.right.contains(msg.X, msg.Y) || m.screen == "log" || m.screen == "create-preview" {
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
