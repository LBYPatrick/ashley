package tui

import (
	"errors"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

type nameEditor struct {
	input     textinput.Model
	id        int64
	sessionID string
	draft     bool
}

func (m *Model) beginName() tea.Cmd {
	e := &nameEditor{input: textinput.New(), draft: m.screen == "compose"}
	name := m.runName
	switch m.screen {
	case "activity":
		row := m.selectedActivity()
		if row.session.ID == "" && row.entry.ID == 0 {
			return nil
		}
		e.id, e.sessionID, name = row.entry.ID, row.session.ID, row.session.Name
		if e.sessionID == "" {
			e.sessionID, name = row.entry.SessionID, row.entry.Name
		}

	case "sessions":
		if m.cursor >= len(m.sessionRows) {
			return nil
		}
		s := m.sessionRows[m.cursor]
		e.sessionID = s.ID
		name = s.Name
	case "history":
		if m.cursor >= len(m.historyRows) {
			return nil
		}
		v := m.historyRows[m.cursor]
		e.id = v.ID
		e.sessionID = v.SessionID
		name = v.Name
	}
	e.input.CharLimit = 120
	e.input.SetValue(name)
	e.input.CursorEnd()
	m.rename = e
	return e.input.Focus()
}

func (m *Model) nameKey(msg tea.KeyMsg) tea.Cmd {
	e := m.rename
	switch msg.String() {
	case "esc":
		m.rename = nil
		return nil
	case "ctrl+c":
		return m.quit()
	case "enter":
		name := strings.Join(strings.Fields(e.input.Value()), " ")
		if e.draft {
			m.runName = name
			m.rename = nil
			return nil
		}
		store, err := m.db()
		if err == nil {
			err = store.Rename(e.id, e.sessionID, name)
			store.Close()
		}
		if err == nil && e.sessionID != "" {
			s, loadErr := m.manager().Load(e.sessionID)
			if loadErr == nil {
				s.Name = name
				err = m.manager().Save(s)
			} else if !errors.Is(loadErr, os.ErrNotExist) {
				err = loadErr
			}
		}
		if err != nil {
			m.status = "Could not save name: " + err.Error()
			return nil
		}
		m.rename = nil
		m.refresh()
		m.status = "Session name saved"
		return nil
	}
	var cmd tea.Cmd
	e.input, cmd = e.input.Update(msg)
	return cmd
}
func (m *Model) nameView(f *frame, a appearance) {
	w := min(66, f.width-4)
	r := rect{(f.width - w) / 2, max(2, (f.height-8)/2), w, 7}
	f.fill(r, a.panel)
	f.box(r, a.border)
	f.section(rect{r.x + 2, r.y + 1, r.w - 4, 1}, m.tr("Session name"), a)
	f.input(rect{r.x + 2, r.y + 2, r.w - 4, 3}, m.rename.input.Value(), "Optional · leave blank for task title", true, a, m.rename.input.Position())
	f.put(r.x+2, r.y+5, a.muted.Render(ansi.Truncate(m.localizedBinding("enter Save")+"   "+m.localizedBinding("esc Cancel"), r.w-4, "…")))
}
