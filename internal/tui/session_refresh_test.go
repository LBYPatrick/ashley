package tui

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LBYPatrick/ashley/internal/sessions"
	tea "github.com/charmbracelet/bubbletea"
)

func finishSessionReads(t *testing.T, m *Model) {
	t.Helper()
	for attempts := 0; len(m.pendingCommands) > 0; attempts++ {
		if attempts > 5 {
			t.Fatal("session reads did not finish")
		}
		commands := m.pendingCommands
		m.pendingCommands = nil
		for _, command := range commands {
			m.acceptSessionDetails(command().(sessionDetailsLoaded))
		}
	}
}

func TestSessionNavigationDoesNotWaitForBackend(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	manager := sessions.Manager{Run: func(args []string, _ string) (string, error) {
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
		return "output for " + strings.Join(args, " "), nil
	}}
	m := newModel(t)
	m.open("sessions")
	m.options.Sessions = &manager
	m.sessionRows = []sessions.Session{{ID: "first", TmuxSession: "first"}, {ID: "last", TmuxSession: "last"}}
	m.sessionAlive = map[string]bool{"first": true, "last": true}
	m.sessionStatusesDirty = false
	m.updatePreview()
	if calls.Load() != 0 || len(m.pendingCommands) != 1 {
		t.Fatal("preview ran in the UI handler")
	}
	command := m.takeCommands()
	result := make(chan tea.Msg, 1)
	go func() { result <- command() }()
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("background read did not start")
	}
	for range 10 {
		m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m.Update(tea.KeyMsg{Type: tea.KeyUp})
	}
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 1 || calls.Load() != 1 || len(m.pendingCommands) != 0 {
		t.Fatal("navigation waited for the backend or started extra reads")
	}
	close(release)
	released = true
	_, next := m.Update(<-result)
	if strings.Contains(m.sessionLog, "first") || next == nil {
		t.Fatal("stale output replaced the selected session's output")
	}
	m.logOffset = 3
	m.Update(next())
	if !strings.Contains(m.sessionLog, "last") || calls.Load() != 2 || m.logOffset != 3 {
		t.Fatal("latest selection was not loaded or scroll position changed", m.sessionLog)
	}
}

func TestSessionResultDoesNotReplaceAnotherScreen(t *testing.T) {
	m := newModel(t)
	m.open("sessions")
	m.sessionRows = []sessions.Session{{ID: "old"}}
	m.sessionBusy = true
	m.open("history")
	m.acceptSessionDetails(sessionDetailsLoaded{log: "stale", alive: map[string]bool{"old": true}})
	if m.sessionBusy || m.sessionLog == "stale" || len(m.pendingCommands) != 0 {
		t.Fatal("result changed an inactive screen or started another read")
	}
}

func TestSessionStatusStartsUnknownAndLoadsInBackground(t *testing.T) {
	calls := 0
	manager := sessions.Manager{Run: func(args []string, _ string) (string, error) {
		calls++
		if args[0] == "list-sessions" {
			return "live\n", nil
		}
		return "ready", nil
	}}
	m := newModel(t)
	m.open("sessions")
	m.options.Sessions = &manager
	m.sessionRows = []sessions.Session{{ID: "live", TmuxSession: "live"}}
	m.updatePreview()
	if calls != 0 || !strings.Contains(m.detailText(), "Checking") {
		t.Fatal("status check ran in the UI handler or unknown status appeared finished")
	}
	if command := m.activate(); command != nil || m.screen != "sessions" {
		t.Fatal("unknown status opened the log")
	}
	finishSessionReads(t, m)
	if calls != 2 || !m.sessionAlive["live"] || m.sessionLog != "ready" {
		t.Fatal("background status and preview did not load", calls, m.sessionLog)
	}
}

func TestPeriodicRefreshDoesNotDiscardSlowOutputForSameSession(t *testing.T) {
	m := newModel(t)
	m.open("sessions")
	m.sessionRows = []sessions.Session{{ID: "selected"}}
	m.sessionLogID, m.sessionLog = "selected", "Loading output…"
	m.sessionBusy = true
	m.sessionRequest = 2
	m.sessionStatusesDirty = true
	m.acceptSessionDetails(sessionDetailsLoaded{request: 1, id: "selected", log: "slow output", alive: map[string]bool{"selected": true}})
	if m.sessionLog != "slow output" || len(m.pendingCommands) != 1 {
		t.Fatal("periodic refresh discarded useful output or did not schedule a new check")
	}
}
