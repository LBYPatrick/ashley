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
	directoryOnly                bool
}

func (m *Model) rememberPlace() {
	if m.places == nil {
		m.places = map[string]browserPlace{}
		m.drafts = map[string]string{}
		m.parents = map[string]string{}
	}
	m.places[m.screen] = browserPlace{
		cursor: m.cursor, offset: m.offset, preview: m.preview.YOffset,
		log: m.logOffset, filter: m.filter.Value(), directoryOnly: m.directoryOnly,
	}
	m.drafts[m.screen] = m.question.Value()
}

func (m *Model) backDestination() string {
	if parent := m.parents[m.screen]; parent != "" {
		return parent
	}
	return "hub"
}

var navigation = []struct{ key, label, screen string }{
	{"ctrl+1", "Home", "hub"}, {"ctrl+2", "New run", "compose"}, {"ctrl+3", "Activity", "activity"},
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
		label := strings.Replace(item.key, "ctrl+", "^", 1) + " " + m.tr(item.label)
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
	scroll           int
}

func (m *Model) requestConfirmation(key string) bool {
	var title, body string
	if entry := m.selectHistoryEntry(); key == "d" && entry.ID != 0 && m.selectedSession().ID == "" {
		title = "Delete this history entry?"
		body = fmt.Sprintf(m.tr("Remove #%d from Ashley history. The agent's conversation remains available in its own storage."), entry.ID)
	}
	if m.screen == "sessions" || m.screen == "activity" {
		if key == "cleanup" {
			title, body = "Remove all finished sessions?", "Remove finished session records in every directory, including hidden results. Keep their logs and all running sessions."
		}
		if key == "X" {
			title, body = "Stop all running sessions?", "Stop all running Ashley sessions in every directory, including hidden results. Remove all session records. Keep their logs. This can interrupt work."
		} else if s := m.selectedSession(); s.ID != "" {
			if key == "K" {
				title, body = "Stop this session?", fmt.Sprintf(m.tr("Stop %s (%s). In-progress work may be interrupted. The log will remain."), s.Skill, s.ID)
			}
			if key == "d" && !m.sessionAlive[s.ID] {
				title, body = "Delete this session and log?", fmt.Sprintf(m.tr("Permanently remove the saved log for %s (%s). This cannot be undone."), s.Skill, s.ID)
			}
		}
	}
	if title == "" {
		return false
	}
	m.confirm = &confirmation{title: m.tr(title), body: m.tr(body), key: key}
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
	case "up", "k":
		m.scrollConfirmation(-1)
	case "down", "j":
		m.scrollConfirmation(1)
	case "pgup":
		m.scrollConfirmation(-m.confirmBodyRect().h)
	case "pgdown", " ":
		m.scrollConfirmation(m.confirmBodyRect().h)
	case "enter":
		c := m.confirm
		m.confirm = nil
		if c.proceed {
			return m.recordAction(c.key)
		}
	}
	return nil
}
