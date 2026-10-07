package tui

import (
	"fmt"
	"strings"

	"github.com/LBYPatrick/ashley/internal/agents"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

type browserPlace struct {
	cursor, offset, preview, log int
	filter                       string
}

func (m *Model) rememberPlace() {
	if m.places == nil {
		m.places = map[string]browserPlace{}
		m.drafts = map[string]string{}
		m.parents = map[string]string{}
	}
	m.places[m.screen] = browserPlace{m.cursor, m.offset, m.preview.YOffset, m.logOffset, m.filter.Value()}
	m.drafts[m.screen] = m.question.Value()
}

func (m *Model) backDestination() string {
	if parent := m.parents[m.screen]; parent != "" {
		return parent
	}
	return "hub"
}

var navigation = []struct{ key, label, screen string }{
	{"ctrl+1", "Home", "hub"}, {"ctrl+2", "New run", "compose"}, {"ctrl+3", "Activity", "sessions"},
	{"ctrl+4", "Library", "skills.sh"}, {"ctrl+5", "Settings", "settings"},
}

func navigationTarget(key string) string {
	for _, item := range navigation {
		if item.key == key {
			return item.screen
		}
	}
	return ""
}

func agentFlag(agent string) string {
	if agent == "claude" {
		return "--claude"
	}
	if agent == "codex" {
		return "--codex"
	}
	return "--" + agent
}

func (m *Model) cycleAgent() {
	keys := agents.Keys()
	for i, key := range keys {
		if key == m.agent {
			m.agent = keys[(i+1)%len(keys)]
			return
		}
	}
}

func (m *Model) navControls() []settingControl {
	var controls []settingControl
	x := 2
	for _, item := range navigation {
		label := strings.Replace(item.key, "ctrl+", "^", 1) + " " + item.label
		if m.width < 76 {
			label = strings.Replace(item.key, "ctrl+", "^", 1)
		}
		w := ansi.StringWidth(label) + 2
		if m.width < 32 {
			label = strings.TrimPrefix(item.key, "ctrl+")
			w = 2
		}
		if x+w > m.width-1 {
			break
		}
		controls = append(controls, settingControl{rect: rect{x, 1, w, 1}, key: item.screen, label: label})
		x += w
		if m.width >= 32 {
			x++
		}
	}
	return controls
}

type confirmation struct {
	title, body, key string
	proceed          bool
}

func (m *Model) requestConfirmation(key string) bool {
	var title, body string
	if m.screen == "history" && key == "d" && len(m.historyRows) > 0 {
		title = "Delete this history entry?"
		body = fmt.Sprintf("Remove #%d from Ashley history. The agent's conversation remains available in its own storage.", m.historyRows[m.cursor].ID)
	}
	if m.screen == "sessions" {
		if key == "cleanup" {
			title, body = "Remove finished sessions?", "Delete saved records and logs for finished sessions. Running sessions will remain."
		}
		if key == "X" && len(m.sessionRows) > 0 {
			title, body = "Stop all running sessions?", "Every active Ashley agent will stop. In-progress work may be interrupted. Session logs will remain."
		} else if len(m.sessionRows) > 0 {
			s := m.sessionRows[m.cursor]
			if key == "K" {
				title, body = "Stop this session?", "Stop "+s.Skill+" ("+s.ID+"). In-progress work may be interrupted. The log will remain."
			}
			if key == "d" && !m.sessionAlive[s.ID] {
				title, body = "Delete this session and log?", "Permanently remove the saved log for "+s.Skill+" ("+s.ID+"). This cannot be undone."
			}
		}
	}
	if title == "" {
		return false
	}
	m.confirm = &confirmation{title: title, body: body, key: key}
	return true
}

func (m *Model) confirmKey(key string) tea.Cmd {
	switch key {
	case "esc", "n":
		m.confirm = nil
	case "ctrl+c":
		return m.quit()
	case "tab", "shift+tab", "left", "right":
		m.confirm.proceed = !m.confirm.proceed
	case "enter":
		c := m.confirm
		m.confirm = nil
		if c.proceed {
			return m.recordAction(c.key)
		}
	}
	return nil
}

func (m *Model) confirmBounds() rect {
	w := min(62, max(20, m.width)-4)
	h := min(max(8, m.height)-2, wrappedLines(m.confirm.body, w-6)+7)
	return rect{(max(20, m.width) - w) / 2, (max(8, m.height) - h) / 2, w, h}
}

func (m *Model) confirmView(f *frame, a appearance) {
	r := m.confirmBounds()
	f.fill(r, a.panel)
	f.box(r, a.border)
	f.put(r.x+2, r.y+1, a.title.Render(ansi.Truncate(m.confirm.title, r.w-4, "…")))
	f.text(rect{r.x + 3, r.y + 3, r.w - 6, max(1, r.h-6)}, m.confirm.body, a.panel, 0)
	for i, label := range []string{"Cancel", "Confirm"} {
		style := a.panel
		if (i == 1) == m.confirm.proceed {
			style = a.selected
		}
		f.put(r.x+3+i*13, r.y+r.h-2, style.Render(" "+label+" "))
	}
}
