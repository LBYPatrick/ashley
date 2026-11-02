package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/LBYPatrick/ashley/internal/history"
	"github.com/LBYPatrick/ashley/internal/sessions"
	tea "github.com/charmbracelet/bubbletea"
)

func TestFinishedSessionEnterUsesLinkedHistoryResume(t *testing.T) {
	m := activitySearchModel(t)
	store, err := m.db()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var selectedID int64
	for _, row := range []history.Invocation{
		{SessionID: "catalog", AgentType: "codex", AgentSessionID: "original", Skill: "debug", CWD: "/workspace/catalog", Timestamp: "2026-11-01T10:00:00Z"},
		{SessionID: "catalog", AgentType: "codex", AgentSessionID: "selected", Skill: "debug", CWD: "/workspace/catalog", Timestamp: "2026-11-01T11:00:00Z"},
		{SessionID: "catalog-copy", AgentType: "codex", AgentSessionID: "unrelated", Skill: "debug", CWD: "/workspace/catalog", Timestamp: "2026-11-02T10:00:00Z"},
	} {
		id, err := store.Record(row)
		if err != nil {
			t.Fatal(err)
		}
		if row.AgentSessionID == "selected" {
			selectedID = id
		}
	}
	m.open("sessions")
	key(m, "/")
	key(m, "catalog")
	key(m, "enter")
	m.sessionAlive = map[string]bool{"catalog": false}
	var args []string
	m.options.Execute = func(argv []string) tea.Cmd { args = argv; return nil }
	key(m, "enter")
	if !reflect.DeepEqual(args, []string{"history", "resume", fmt.Sprint(selectedID)}) {
		t.Fatal("finished session did not use its exact history entry", args)
	}
	if m.screen != "sessions" || m.filter.Value() != "catalog" || !strings.Contains(m.detailText(), "Enter to resume") {
		t.Fatal("resume lost the session browser or still offers log viewing")
	}
	if !strings.Contains(m.detailText(), "L opens the log") {
		t.Fatal("session details hide explicit log access")
	}
}

func TestSessionEnterWithoutLinkedHistoryDoesNotGuess(t *testing.T) {
	for _, sessionID := range []string{"missing", ""} {
		t.Run(sessionID, func(t *testing.T) {
			m := newModel(t)
			store, err := m.db()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.Record(history.Invocation{Skill: "raw", SessionID: "another", CWD: "/same", AgentSessionID: "wrong"}); err != nil {
				t.Fatal(err)
			}
			store.Close()
			m.open("sessions")
			m.sessionRows = []sessions.Session{{ID: sessionID, CWD: "/same"}}
			m.sessionAlive = map[string]bool{sessionID: false}
			called := false
			m.options.Execute = func([]string) tea.Cmd { called = true; return nil }
			key(m, "enter")
			if called || m.screen != "sessions" || !strings.Contains(m.status, "Press L") {
				t.Fatal("unlinked session resumed another conversation or opened its log", m.status)
			}
		})
	}
}

func TestSessionEnterAttachesRunningAndWaitsForUnknownStatus(t *testing.T) {
	m := newModel(t)
	m.open("sessions")
	m.sessionRows = []sessions.Session{{ID: "running"}}
	var args []string
	m.options.Execute = func(argv []string) tea.Cmd { args = argv; return nil }
	key(m, "enter")
	if args != nil {
		t.Fatal("unknown session status started a command", args)
	}
	m.sessionAlive = map[string]bool{"running": true}
	key(m, "enter")
	if !reflect.DeepEqual(args, []string{"attach", "running"}) {
		t.Fatal("running session did not attach", args)
	}
}

func TestSessionResumeReportsHistoryReadFailure(t *testing.T) {
	m := newModel(t)
	m.open("sessions")
	m.sessionRows = []sessions.Session{{ID: "finished"}}
	m.sessionAlive = map[string]bool{"finished": false}
	path := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(path, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	m.options.Home = path
	t.Setenv("XDG_DATA_HOME", path)
	called := false
	m.options.Execute = func([]string) tea.Cmd { called = true; return nil }
	key(m, "enter")
	if called || !strings.Contains(m.status, path) || m.screen != "sessions" {
		t.Fatal("history failure launched a command or opened a log", m.status)
	}
}
