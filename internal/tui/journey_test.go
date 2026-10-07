package tui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/LBYPatrick/ashley/internal/history"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestComposerJourneyKeepsDraftAndLaunchesSelectedOptions(t *testing.T) {
	trueColor(t)
	m := newModel(t)
	var got []string
	m.options.Execute = func(args []string) tea.Cmd { got = args; return nil }
	m.Update(tabShortcut(2))
	key(m, "ctrl+r")
	if len(got) > 0 || !strings.Contains(m.status, "Describe a task") {
		t.Fatal("empty direct run launched")
	}
	key(m, "Build an API")
	key(m, "enter")
	key(m, "Keep existing routes")
	key(m, "ctrl+s")
	key(m, "/")
	key(m, "find and fix bugs")
	key(m, "enter")
	key(m, "enter")
	if m.screen != "compose" || m.runSkill != "debug" {
		t.Fatal(m.screen, m.runSkill)
	}
	key(m, "tab")
	key(m, "enter") // Select Codex.
	key(m, "tab")
	key(m, "right") // Full access.
	m.Update(tabShortcut(4))
	m.Update(tabShortcut(2))
	if m.composer.Value() != "Build an API\nKeep existing routes" || m.runSkill != "debug" || m.agent != "codex" || m.mode != 1 {
		t.Fatal("navigation lost the run draft")
	}
	key(m, "ctrl+r")
	want := []string{"run", "--codex", "--dangerously-skip-permissions", "debug", "--", "Build an API\nKeep existing routes"}
	if !reflect.DeepEqual(got, want) {
		t.Fatal(got, want)
	}
	key(m, "ctrl+d")
	if m.runSkill != "raw" {
		t.Fatal("cannot remove selected skill")
	}
	exportRegressionView(t, "composer", m.View())
}

func TestSkillSearchPrioritizesNamesAndBackReturnsToLibrary(t *testing.T) {
	m := newModel(t)
	m.open("skills.sh")
	m.activate()
	key(m, "/")
	key(m, "a-debug")
	if len(m.names) == 0 || m.names[0] != "debug" {
		t.Fatal("exact skill name was not first", m.names)
	}
	key(m, "esc")
	key(m, "esc")
	if m.screen != "skills.sh" {
		t.Fatal("skill chooser did not return to Library")
	}
	m.open("create")
	key(m, "esc")
	if m.screen != "skills.sh" {
		t.Fatal("creator did not return to Library")
	}
}

func TestActivityRestoresSearchAndConfirmationDefaultsToCancel(t *testing.T) {
	m := newModel(t)
	store, err := m.db()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, q := range []string{"Fix login", "Fix settings", "Add API"} {
		if _, err := store.Record(history.Invocation{Skill: "feat", Question: q}); err != nil {
			t.Fatal(err)
		}
	}
	m.open("history")
	key(m, "/")
	key(m, "Fix")
	key(m, "enter")
	key(m, "down")
	id := m.historyRows[m.cursor].ID
	key(m, "t")
	key(m, "t")
	if m.filter.Value() != "Fix" || m.historyRows[m.cursor].ID != id {
		t.Fatal("activity lost its place")
	}
	key(m, "d")
	if m.confirm == nil || m.confirm.proceed {
		t.Fatal("delete did not default to cancel")
	}
	key(m, "enter")
	if _, err := store.Get(id); err != nil {
		t.Fatal("cancel removed entry", err)
	}
	key(m, "d")
	key(m, "tab")
	exportRegressionView(t, "confirmation", m.View())
	key(m, "esc")
	if _, err := store.Get(id); err != nil {
		t.Fatal("escape removed entry", err)
	}
	key(m, "d")
	key(m, "tab")
	key(m, "enter")
	if _, err := store.Get(id); err == nil {
		t.Fatal("confirmed delete did not remove entry")
	}
}

func TestResponsiveWorkspacesAndRecentResume(t *testing.T) {
	trueColor(t)
	m := newModel(t)
	store, err := m.db()
	if err != nil {
		t.Fatal(err)
	}
	id, err := store.Record(history.Invocation{Skill: "feat", Question: "Build the new settings page", AgentType: "codex", AgentSessionID: "conversation-42", CWD: "/workspace/ashley"})
	if err != nil {
		t.Fatal(err)
	}
	store.Close()
	var got []string
	m.options.Execute = func(args []string) tea.Cmd { got = args; return nil }
	for _, size := range [][2]int{{50, 20}, {80, 24}, {120, 36}, {180, 48}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for _, screen := range []string{"hub", "compose", "vibe", "sessions", "history", "skills.sh", "settings", "create"} {
			m.open(screen)
			assertFrame(t, m.View(), size[0], size[1])
			if size[0] == 120 {
				exportRegressionView(t, "workspace-"+strings.ReplaceAll(screen, ".", "-"), m.View())
			}
		}
		m.open("history")
		if size[0] < 76 {
			if m.layout().detail.w != 0 {
				t.Fatal("narrow screen squeezed two columns")
			}
			key(m, "v")
			if !strings.Contains(ansi.Strip(m.View()), "Enter to resume") {
				t.Fatal("narrow detail unavailable")
			}
			key(m, "esc")
			if m.screen != "history" || m.compactDetail {
				t.Fatal("detail back lost list")
			}
		}
	}
	m.open("hub")
	m.cursor = len(hub)
	key(m, "enter")
	if !reflect.DeepEqual(got, []string{"history", "resume", "1"}) || id != 1 {
		t.Fatal(got, id)
	}
}

func TestNewNavigationPreservesCreatorDraft(t *testing.T) {
	m := newModel(t)
	m.open("create")
	wizardText(m, "draft-name")
	w := m.wizard
	m.Update(tabShortcut(2))
	m.open("create")
	if m.wizard != w || m.wizard.input.Value() != "draft-name" {
		t.Fatal("workspace navigation lost creator draft")
	}
}
