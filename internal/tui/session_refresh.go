package tui

import (
	"maps"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type sessionDetailsLoaded struct {
	request uint64
	id      string
	alive   map[string]bool
	log     string
}

func (m *Model) queueSessionDetails() {
	if m.sessionBusy || m.screen != "sessions" || len(m.sessionRows) == 0 {
		return
	}
	manager, rows := m.manager(), slices.Clone(m.sessionRows)
	selected, request := rows[m.cursor], m.sessionRequest
	alive, checkStatus := maps.Clone(m.sessionAlive), m.sessionStatusesDirty
	if _, known := alive[selected.ID]; !known {
		checkStatus = true
	}
	m.sessionBusy, m.sessionStatusesDirty = true, false
	// Commands use snapshots. Only Update changes the model, and cursor moves
	// replace the pending selection instead of starting more backend processes.
	m.pendingCommands = append(m.pendingCommands, func() tea.Msg {
		if checkStatus {
			alive = manager.AliveAll(rows)
		}
		content, err := manager.Preview(selected, 50, alive[selected.ID])
		if err != nil {
			content = err.Error()
		}
		if strings.TrimSpace(content) == "" {
			content = "(empty log)"
		}
		return sessionDetailsLoaded{request: request, id: selected.ID, alive: alive, log: content}
	})
}

func (m *Model) acceptSessionDetails(result sessionDetailsLoaded) {
	m.sessionBusy = false
	if m.screen != "sessions" || len(m.sessionRows) == 0 {
		return
	}
	m.sessionAlive = result.alive
	// A periodic refresh can arrive before a slow read finishes. Its output is
	// still useful for the same session; reject only a different selection.
	if result.id == m.sessionRows[m.cursor].ID {
		m.sessionLog = result.log
	}
	offset := m.preview.YOffset
	m.setPreviewContent()
	m.preview.SetYOffset(offset)
	if result.request != m.sessionRequest || m.sessionStatusesDirty {
		m.queueSessionDetails()
	}
}
