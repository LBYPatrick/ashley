package tui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/LBYPatrick/ashley/internal/agents"
	"github.com/LBYPatrick/ashley/internal/history"
	"github.com/LBYPatrick/ashley/internal/sessions"
	tea "github.com/charmbracelet/bubbletea"
)

type activityRow struct {
	session sessions.Session
	entry   history.Invocation
	when    time.Time
}

func (r activityRow) title() string {
	if r.session.ID != "" {
		return taskTitle(r.session.Skill, r.session.Question, r.session.Name)
	}
	return taskTitle(r.entry.Skill, r.entry.Question, r.entry.Name)
}

func (r activityRow) cwd() string {
	if r.session.CWD != "" {
		return r.session.CWD
	}
	return r.entry.CWD
}

func (r activityRow) skill() string {
	if r.session.Skill != "" {
		return r.session.Skill
	}
	return r.entry.Skill
}

func (r activityRow) timestamp() string {
	if r.session.StartedAt != "" {
		return r.session.StartedAt
	}
	return r.entry.Timestamp
}

func mergeActivity(sessionRows []sessions.Session, historyRows []history.Invocation) []activityRow {
	rows := make([]activityRow, 0, len(sessionRows)+len(historyRows))
	bySession := make(map[string]int)
	for _, session := range sessionRows {
		if session.Superseded {
			continue
		}
		bySession[session.ID] = len(rows)
		when, _ := time.Parse(time.RFC3339Nano, session.StartedAt)
		rows = append(rows, activityRow{session: session, when: when})
	}
	for _, entry := range historyRows {
		if index, ok := bySession[entry.SessionID]; ok && entry.SessionID != "" {
			if rows[index].entry.ID == 0 {
				rows[index].entry = entry
				if rows[index].when.IsZero() {
					rows[index].when, _ = time.Parse(time.RFC3339Nano, entry.Timestamp)
				}
			}
			continue
		}
		when, _ := time.Parse(time.RFC3339Nano, entry.Timestamp)
		if entry.SessionID != "" {
			bySession[entry.SessionID] = len(rows)
		}
		rows = append(rows, activityRow{entry: entry, when: when})
	}
	slices.SortStableFunc(rows, func(a, b activityRow) int {
		return b.when.Compare(a.when)
	})
	return rows
}

func (m *Model) refreshActivity() {
	manager := m.manager()
	sessionRows, err := manager.All()
	if err != nil {
		m.status = err.Error()
	}
	// Keep the last database snapshot on a failed read.
	if store, err := m.db(); err != nil {
		m.status = err.Error()
	} else {
		entries, err := store.Query(history.Filter{}, -1, 0)
		store.Close()
		if err == nil {
			m.historyRows = entries
		} else {
			m.status = err.Error()
		}
	}
	previous := m.selectedActivity()
	query := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	m.activityRows = m.activityRows[:0]
	rows := mergeActivity(sessionRows, m.historyRows)
	if m.sortMode == "skill" {
		slices.SortStableFunc(rows, func(a, b activityRow) int {
			return strings.Compare(strings.ToLower(a.skill()), strings.ToLower(b.skill()))
		})
	}
	for _, row := range rows {
		fields := []string{row.cwd()}
		if !m.directoryOnly {
			fields = append(fields, row.title(), row.session.Question, row.session.Skill, row.session.Agent, row.session.ID, row.session.CWD,
				row.entry.Question, row.entry.Name, row.entry.AgentType, row.entry.Skill, row.entry.CWD, row.entry.SessionID, row.entry.AgentSessionID)
		}
		if slices.ContainsFunc(fields, func(field string) bool { return strings.Contains(strings.ToLower(field), query) }) {
			if (previous.session.ID != "" && row.session.ID == previous.session.ID) || (previous.entry.ID != 0 && row.entry.ID == previous.entry.ID) {
				m.cursor = len(m.activityRows)
			}
			m.activityRows = append(m.activityRows, row)
		}
	}
	m.activitySessions = sessionRows
	m.sessionStatusesDirty = true
}

func (m *Model) selectedActivity() activityRow {
	if m.cursor < 0 || m.cursor >= len(m.activityRows) {
		return activityRow{}
	}
	return m.activityRows[m.cursor]
}

func (m *Model) selectedSession() sessions.Session {
	if m.screen == "activity" {
		return m.selectedActivity().session
	}
	if m.screen == "sessions" && m.cursor >= 0 && m.cursor < len(m.sessionRows) {
		return m.sessionRows[m.cursor]
	}
	return sessions.Session{}
}

func (m *Model) selectHistoryEntry() history.Invocation {
	if m.screen == "activity" {
		return m.selectedActivity().entry
	}
	if m.screen == "history" && m.cursor >= 0 && m.cursor < len(m.historyRows) {
		return m.historyRows[m.cursor]
	}
	return history.Invocation{}
}

func (m *Model) activityStatus(row activityRow) (symbol, label string) {
	if row.session.ID != "" {
		if alive, known := m.sessionAlive[row.session.ID]; !known {
			return "?", m.tr("Checking session status…")
		} else if alive {
			return "●", m.tr("Running · Enter to attach")
		}
	}
	if row.entry.ID == 0 {
		return "○", m.tr("Finished")
	}
	return "○", m.tr("Finished · Enter to resume")
}

func (m *Model) activityDetail() string {
	row := m.selectedActivity()
	if row.session.ID == "" && row.entry.ID == 0 {
		return m.tr("Nothing here yet.")
	}
	_, state := m.activityStatus(row)
	agent, skill, id := row.entry.AgentType, row.entry.Skill, row.entry.SessionID
	if row.session.ID != "" {
		agent, skill, id = row.session.Agent, row.session.Skill, row.session.ID
	}
	text := row.title() + "\n\n" + state + "\n" + agents.Get(agent).Label + " · " + skill + "\n\n" + row.cwd()
	if m.showMetadata {
		text += "\n\n" + m.tr("Session ID") + ": " + id + "\n" + m.tr("History") + ": "
		if row.entry.ID != 0 {
			text += fmt.Sprintf("#%d", row.entry.ID)
		} else {
			text += "—"
		}
		text += "\n" + row.timestamp()
		if row.entry.AgentSessionID != "" {
			text += "\n" + m.tr("Conversation ID") + ": " + row.entry.AgentSessionID
		}
	}
	return text
}

func (m *Model) activateActivity() tea.Cmd {
	row := m.selectedActivity()
	if row.session.ID != "" {
		alive, known := m.sessionAlive[row.session.ID]
		if !known {
			return nil
		}
		if alive {
			return m.execute("attach", row.session.ID)
		}
	}
	if row.entry.ID != 0 {
		return m.execute("history", "resume", fmt.Sprint(row.entry.ID))
	}
	if row.session.ID != "" {
		return m.resumeSession(row.session.ID)
	}
	return nil
}
