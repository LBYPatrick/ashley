package tui

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSkillsActions(t *testing.T) {
	for _, tc := range []struct {
		index int
		input string
		want  []string
	}{
		{0, "animations", []string{"__tui-command", "skills", "find", "animations"}},
		{1, "owner/repo with spaces", []string{"__tui-command", "skills", "add", "owner/repo with spaces", "--global"}},
		{2, "", []string{"__tui-command", "skills", "list", "--global"}},
		{3, "", []string{"__tui-command", "skills", "remove", "--global"}},
		{4, "", []string{"__tui-command", "skills", "check"}},
		{5, "", []string{"__tui-command", "skills", "update"}},
		{6, "", []string{"__tui-command", "install", "--skills-only"}},
	} {
		t.Run(skillsActions[tc.index].label, func(t *testing.T) {
			m := newModel(t)
			m.open("skills.sh")
			m.cursor = tc.index + 3
			var got []string
			m.options.Execute = func(args []string) tea.Cmd { got = args; return nil }
			if tc.index < 2 {
				m.activate()
				m.question.SetValue(tc.input)
			}
			key(m, "enter")
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestSkillsInputValidationAndNavigation(t *testing.T) {
	m := newModel(t)
	m.open("skills.sh")
	m.cursor = 4
	m.activate()
	for _, input := range []string{"", "--all"} {
		m.question.SetValue(input)
		if cmd := m.activateSkills(); cmd != nil || m.status == "" {
			t.Fatal("invalid input accepted")
		}
	}
	key(m, "esc")
	if m.focus != "" || m.screen != "skills.sh" {
		t.Fatal("input escape")
	}
	key(m, "esc")
	if m.screen != "hub" {
		t.Fatal("page escape")
	}
	m.cursor = len(hub) - 1
	m.activate()
	if m.screen != "skills.sh" || !strings.Contains(m.View(), "Find skills") {
		t.Fatal("hub navigation")
	}
	m.open("hub")
	m.paletteOpen = true
	m.paletteQuery = "Skills.sh"
	m.paletteCursor = 0
	m.paletteKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != "skills.sh" {
		t.Fatal("palette navigation")
	}
}
