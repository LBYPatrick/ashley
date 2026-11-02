package tui

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/LBYPatrick/ashley/internal/history"
	"github.com/LBYPatrick/ashley/internal/sessions"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func activitySearchModel(t *testing.T) *Model {
	t.Helper()
	m := newModel(t)
	manager := m.manager()
	manager.Run = func([]string, string) (string, error) { return "", nil }
	m.options.Sessions = &manager
	for _, row := range []sessions.Session{
		{ID: "billing", Name: "Add payment reports", Skill: "feat", CWD: "/workspace/billing", StartedAt: "2026-10-02T00:00:00Z"},
		{ID: "catalog", Name: "Fix billing links", Skill: "debug", CWD: "/workspace/catalog", StartedAt: "2026-10-01T00:00:00Z"},
	} {
		row.TmuxSession, row.LogFile = row.ID, manager.Dir+"/"+row.ID+".log"
		if err := manager.Save(row); err != nil {
			t.Fatal(err)
		}
	}
	return m
}

func TestActivitySearchScopesAndTabState(t *testing.T) {
	m := activitySearchModel(t)
	store, err := m.db()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for i := range 56 {
		if _, err := store.Record(history.Invocation{Skill: "feat", Question: fmt.Sprintf("Report %d", i), CWD: "/workspace/billing"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.Record(history.Invocation{Skill: "debug", Question: "Fix billing links", CWD: "/workspace/catalog"}); err != nil {
		t.Fatal(err)
	}
	m.open("sessions")
	key(m, "/")
	key(m, "billing")
	if m.focus != "filter" || len(m.sessionRows) != 2 {
		t.Fatal("session search does not include task text and directories")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	if !m.directoryOnly || len(m.sessionRows) != 1 || m.sessionRows[0].ID != "billing" || m.focus != "filter" {
		t.Fatal("directory scope did not limit session results or retain input focus")
	}
	key(m, "enter")
	key(m, "s")
	if m.sortMode != "skill" || len(m.sessionRows) != 1 {
		t.Fatal("sorting removed the session filter")
	}
	m.Update(tick(time.Now()))
	if m.filter.Value() != "billing" || len(m.sessionRows) != 1 || m.sessionRows[0].ID != "billing" {
		t.Fatal("refresh removed the session filter")
	}
	key(m, "t")
	if m.filter.Value() != "" || m.directoryOnly {
		t.Fatal("session search leaked into history")
	}
	key(m, "/")
	key(m, "billing")
	key(m, "enter")
	if m.total != 57 {
		t.Fatal("general history search excludes matching tasks", m.total)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	if m.total != 56 || len(m.historyRows) != 50 {
		t.Fatal("directory history query and count disagree", m.total, len(m.historyRows))
	}
	key(m, "n")
	key(m, "down")
	if m.offset != 50 || len(m.historyRows) != 6 || m.cursor != 1 {
		t.Fatal("directory results did not paginate")
	}
	selected := m.historyRows[m.cursor].ID
	key(m, "t")
	if m.filter.Value() != "billing" || !m.directoryOnly || len(m.sessionRows) != 1 {
		t.Fatal("session tab did not retain its query and scope")
	}
	key(m, "t")
	if !m.directoryOnly || m.offset != 50 || m.cursor != 1 || m.historyRows[m.cursor].ID != selected {
		t.Fatal("history tab did not retain its query, scope, and selection")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	if m.offset != 0 || m.cursor != 0 || m.total != 57 {
		t.Fatal("scope change did not reset pagination and selection")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	if m.filter.Value() != "" || m.total != 57 {
		t.Fatal("clear shortcut did not restore all results")
	}
}

func TestSessionSearchRejectsStaleOutputAndEmptyActions(t *testing.T) {
	m := activitySearchModel(t)
	m.open("sessions")
	oldID, request := m.sessionRows[m.cursor].ID, m.sessionRequest
	key(m, "/")
	key(m, "catalog")
	if len(m.sessionRows) != 1 || m.sessionRows[0].ID == oldID {
		t.Fatal("search did not select another session")
	}
	m.acceptSessionDetails(sessionDetailsLoaded{request: request, id: oldID, log: "stale output", alive: map[string]bool{oldID: true}})
	if m.sessionLog == "stale output" || len(m.pendingCommands) == 0 {
		t.Fatal("stale output replaced the filtered selection or its new read was not queued")
	}
	key(m, "impossible")
	key(m, "enter")
	if len(m.sessionRows) != 0 || m.cursor != 0 || m.sessionLog != "" {
		t.Fatal("empty results retained the previous session output")
	}
	if cmd := m.activate(); cmd != nil {
		t.Fatal("empty search results started an action")
	}
	m.acceptSessionDetails(sessionDetailsLoaded{request: request, id: oldID, log: "stale output"})
	if m.sessionBusy || m.sessionLog != "" {
		t.Fatal("late output replaced empty search results")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	if len(m.sessionRows) != 2 || m.cursor != 0 {
		t.Fatal("clearing empty search did not restore sessions")
	}
}

func TestSessionLogReturnsToFilteredSelection(t *testing.T) {
	m := activitySearchModel(t)
	m.open("sessions")
	key(m, "/")
	key(m, "workspace")
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	key(m, "enter")
	key(m, "down")
	m.preview.SetYOffset(2)
	m.logOffset = 1
	selected, detailOffset := m.sessionRows[m.cursor], m.preview.YOffset
	if err := os.WriteFile(selected.LogFile, []byte("saved session output"), 0600); err != nil {
		t.Fatal(err)
	}
	m.sessionAlive = map[string]bool{selected.ID: false}
	key(m, "l")
	if m.screen != "log" || !strings.Contains(m.logContent, "saved session output") {
		t.Fatal("session action did not open the log")
	}
	key(m, "esc")
	if m.screen != "sessions" || m.filter.Value() != "workspace" || !m.directoryOnly || m.cursor != 1 {
		t.Fatal("return from log lost the search, scope, or selection")
	}
	if m.sessionRows[m.cursor].ID != selected.ID || m.preview.YOffset != detailOffset || m.logOffset != 1 {
		t.Fatal("return from log lost the selected session or scroll position")
	}
}

func TestKillAllWithEmptySearchResultsRequiresConfirmation(t *testing.T) {
	for _, route := range []string{"keyboard", "mouse", "palette"} {
		t.Run(route, func(t *testing.T) {
			m := activitySearchModel(t)
			killed := 0
			m.options.Sessions.Run = func(args []string, _ string) (string, error) {
				if args[0] == "kill-session" {
					killed++
				}
				return "", nil
			}
			m.open("sessions")
			key(m, "/")
			key(m, "missing")
			m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
			key(m, "enter")
			if len(m.sessionRows) != 0 {
				t.Fatal("test search did not hide all sessions")
			}
			trigger := func() {
				switch route {
				case "keyboard":
					key(m, "X")
				case "mouse":
					clickSessionBulkAction(t, m, "X")
				case "palette":
					m.paletteOpen, m.paletteQuery, m.paletteCursor = true, "Kill all sessions", 0
					key(m, "enter")
				}
			}
			trigger()
			if m.confirm == nil || m.confirm.key != "X" || m.confirm.proceed || !strings.Contains(m.confirm.body, "hidden results") {
				t.Fatal("empty search did not require confirmation for hidden sessions")
			}
			if rows, err := m.manager().All(); err != nil || len(rows) != 2 || killed != 0 {
				t.Fatal("hidden sessions changed before confirmation", rows, err, killed)
			}
			key(m, "enter")
			if rows, err := m.manager().All(); err != nil || len(rows) != 2 || killed != 0 {
				t.Fatal("cancel changed hidden sessions", rows, err, killed)
			}
			trigger()
			key(m, "tab")
			key(m, "enter")
			if rows, err := m.manager().All(); err != nil || len(rows) != 0 || killed != 2 {
				t.Fatal("confirmed action did not stop all hidden sessions", rows, err, killed)
			}
		})
	}
}

func TestActivitySearchControlsAndSmallLayouts(t *testing.T) {
	for _, size := range [][2]int{{20, 8}, {30, 10}, {50, 12}, {50, 16}, {50, 24}, {100, 30}, {160, 40}} {
		for _, screen := range []string{"sessions", "history"} {
			t.Run(fmt.Sprintf("%s-%dx%d", screen, size[0], size[1]), func(t *testing.T) {
				m := activitySearchModel(t)
				m.open(screen)
				m.width, m.height = size[0], size[1]
				if screen == "history" {
					m.historyRows = []history.Invocation{{ID: 1, Name: "Add payment reports", Skill: "feat", CWD: "/workspace/billing"}}
					m.total = 1
				}
				view := m.View()
				assertFrame(t, view, size[0], size[1])
				exportRegressionView(t, fmt.Sprintf("activity-%s-%dx%d", screen, size[0], size[1]), view)
				l := m.layout()
				if l.list.y <= l.input.y || l.list.y+l.list.h > m.height-2 {
					t.Fatal("search or results overlap terminal chrome", l)
				}
				if m.rowHeight() >= 2 {
					listRows := strings.Split(ansi.Strip(view), "\n")[l.list.y : l.list.y+m.rowHeight()]
					if !strings.Contains(strings.Join(listRows, "\n"), "billing") {
						t.Fatal("working directory is hidden in the result list")
					}
				}
				m.mouse(tea.MouseMsg{X: l.input.x + 2, Y: l.input.y + l.input.h/2, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
				if m.focus != "filter" {
					t.Fatal("search field mouse target does not match its visible field")
				}
				key(m, "missing")
				controls := m.activitySearchControls()
				for _, control := range controls {
					if !l.left.contains(control.x, control.y) || control.x+control.w > l.left.x+l.left.w {
						t.Fatal("search control escapes its panel", control)
					}
				}
				m.mouse(tea.MouseMsg{X: controls[0].x, Y: controls[0].y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
				view = m.View()
				if !m.directoryOnly || !strings.Contains(ansi.Strip(view), m.activitySearchControls()[0].label) || len(controls) != 2 {
					t.Fatal("empty results hide the active scope or clear control")
				}
				if m.height >= 13 && !strings.Contains(ansi.Strip(view), "No matching results.") {
					t.Fatal("filtered empty state suggests no saved records exist")
				}
				assertFrame(t, view, size[0], size[1])
				exportRegressionView(t, fmt.Sprintf("activity-%s-empty-%dx%d", screen, size[0], size[1]), view)
				m.mouse(tea.MouseMsg{X: controls[1].x, Y: controls[1].y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
				if m.filter.Value() != "" || !m.directoryOnly || m.focus != "filter" {
					t.Fatal("clear control did not retain scope and input focus")
				}
			})
		}
	}
}
