package tui

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/LBYPatrick/ashley/internal/history"
	"github.com/LBYPatrick/ashley/internal/sessions"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestUnifiedActivityMergesRecordsAndRoutesActions(t *testing.T) {
	m := newModel(t)
	manager := m.manager()
	manager.Run = func(args []string, _ string) (string, error) {
		if args[0] == "list-sessions" {
			return "active\n", nil
		}
		return "", nil
	}
	m.options.Sessions = &manager
	for _, row := range []sessions.Session{
		{ID: "superseded", Superseded: true, Name: "Superseded task", TmuxSession: "superseded"},
		{ID: "live", Name: "Live task", TmuxSession: "active", StartedAt: "2026-01-02T00:00:00Z", CWD: "/work/live"},
		{ID: "finished", Name: "Finished task", TmuxSession: "stopped", StartedAt: "2026-01-01T00:00:00Z", CWD: "/work/stopped"},
	} {
		row.LogFile = filepath.Join(t.TempDir(), row.ID+".log")
		if err := manager.Save(row); err != nil {
			t.Fatal(err)
		}
	}
	store, err := m.db()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, row := range []history.Invocation{
		{SessionID: "live", Question: "Linked searchable text", Timestamp: "2026-01-02T00:00:00Z"},
		{SessionID: "finished", Question: "Earlier task", Timestamp: "2026-01-01T00:00:00Z"},
		{SessionID: "past", Name: "Past task", Timestamp: "2025-12-31T00:00:00Z", CWD: "/work/past"},
		{SessionID: "past", Name: "Earlier duplicate", Timestamp: "2025-12-30T00:00:00Z"},
	} {
		if _, err := store.Record(row); err != nil {
			t.Fatal(err)
		}
	}
	var called []string
	m.options.Execute = func(args []string) tea.Cmd { called = args; return nil }
	_, load := m.Update(tabShortcut(3))
	if m.screen != "activity" || len(m.activityRows) != 3 {
		t.Fatalf("expected a single view with one row per record: %s %+v", m.screen, m.activityRows)
	}
	if load == nil {
		t.Fatal("activity did not schedule a status check")
	}
	m.Update(load())
	if view := ansi.Strip(m.View()); !strings.Contains(view, "● Live task") || !strings.Contains(view, "○ Finished task") || !strings.Contains(view, "○ Past task") {
		t.Fatal("missing combined statuses", view)
	}
	m.activate()
	if !reflect.DeepEqual(called, []string{"attach", "live"}) {
		t.Fatalf("running session action: %v", called)
	}
	m.cursor = 1
	m.activate()
	if !reflect.DeepEqual(called, []string{"history", "resume", "2"}) {
		t.Fatalf("finished session action: %v", called)
	}
	m.cursor = 2
	m.activate()
	if !reflect.DeepEqual(called, []string{"history", "resume", "3"}) {
		t.Fatalf("history-only action: %v", called)
	}
	m.filter.SetValue("linked searchable")
	m.applySearch()
	if len(m.activityRows) != 1 || m.selectedActivity().session.ID != "live" {
		t.Fatal("combined search lost linked history text")
	}
	m.directoryOnly = true
	m.filter.SetValue("/work/past")
	m.applySearch()
	if len(m.activityRows) != 1 || m.selectedActivity().entry.ID != 3 {
		t.Fatal("directory scope did not search history")
	}
}
