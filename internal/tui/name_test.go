package tui

import (
	"strings"
	"testing"

	"github.com/LBYPatrick/ashley/internal/history"
	"github.com/LBYPatrick/ashley/internal/sessions"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestRenamePersistsAcrossActivityAndHistory(t *testing.T) {
	m := newModel(t)
	s, err := m.manager().Prepare(sessions.Session{Skill: "raw", Question: "original prompt"})
	if err != nil {
		t.Fatal(err)
	}
	if err = m.manager().Save(s); err != nil {
		t.Fatal(err)
	}
	db, err := m.db()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	id, err := db.Record(history.Invocation{SessionID: s.ID, Skill: s.Skill, Question: s.Question, AgentSessionID: "stable-conversation"})
	if err != nil {
		t.Fatal(err)
	}
	m.open("history")
	key(m, "N")
	key(m, "Release checklist")
	key(m, "enter")
	v, err := db.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := m.manager().Load(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if v.Name != "Release checklist" || saved.Name != v.Name || v.Question != s.Question || v.AgentSessionID != "stable-conversation" {
		t.Fatal(v, saved)
	}
	found, err := db.Query(history.Filter{Search: "checklist"}, 10, 0)
	if err != nil || len(found) != 1 {
		t.Fatal(found, err)
	}
	key(m, "N")
	m.rename.input.SetValue("discard")
	key(m, "esc")
	v, _ = db.Get(id)
	if v.Name != "Release checklist" {
		t.Fatal(v)
	}
	key(m, "N")
	m.rename.input.SetValue("")
	key(m, "enter")
	v, _ = db.Get(id)
	if v.Name != "" {
		t.Fatal(v)
	}
}
func TestAutoHierarchyAndSharedAlignment(t *testing.T) {
	trueColor(t)
	old := lipgloss.HasDarkBackground()
	defer lipgloss.SetHasDarkBackground(old)
	for _, dark := range []bool{true, false} {
		lipgloss.SetHasDarkBackground(dark)
		m := newModel(t)
		m.width = 120
		m.height = 36
		m.theme.Mode = "auto"
		m.recent = []history.Invocation{{Name: "Prepare October release", Skill: "debug", Question: "original", AgentType: "codex"}}
		left, right := m.homeBounds()
		if left.x != m.composeBounds().x || left.x != m.readingRect().x || left.x != m.settingsBounds().x || right.y != left.y {
			t.Fatal("inconsistent grid")
		}
		a := m.appearance()
		if a.base.GetForeground() == a.muted.GetForeground() || a.border.GetForeground() == a.base.GetForeground() {
			t.Fatal("missing contrast tiers")
		}
		view := m.View()
		if !strings.Contains(view, "RECENT CONVERSATIONS") {
			t.Fatal(view)
		}
		name := "auto-dark"
		if !dark {
			name = "auto-light"
		}
		exportRegressionView(t, name, view)
	}
}

func TestNamedDraftLaunchAndEditor(t *testing.T) {
	m := newModel(t)
	m.theme.Mode = "auto"
	m.open("compose")
	key(m, "ctrl+n")
	key(m, "Release review")
	if !strings.Contains(m.View(), "Session name") {
		t.Fatal("missing name editor")
	}
	key(m, "enter")
	m.composer.SetValue("Review the changes")
	var got []string
	m.options.Execute = func(args []string) tea.Cmd { got = args; return nil }
	key(m, "ctrl+r")
	if !strings.Contains(strings.Join(got, "|"), "--name=Release review|raw|--|Review the changes") {
		t.Fatal(got)
	}
}
