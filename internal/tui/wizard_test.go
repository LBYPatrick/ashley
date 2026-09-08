package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LBYPatrick/ashley/internal/skills"
	tea "github.com/charmbracelet/bubbletea"
)

func wizardPress(m *Model, kind tea.KeyType) { m.Update(tea.KeyMsg{Type: kind}) }
func wizardText(m *Model, text string)       { m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text)}) }

func TestGuidedCreatorEndToEnd(t *testing.T) {
	m := newModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 32})
	m.open("create")
	w := m.wizard
	if w == nil || !strings.Contains(m.View(), "Step 1/4") {
		t.Fatal(m.View())
	}
	wizardPress(m, tea.KeyCtrlN)
	if w.stage != 0 || !strings.Contains(m.status, "enter a name") {
		t.Fatal(w.stage, m.status)
	}
	wizardText(m, "guided")
	wizardPress(m, tea.KeyTab)
	wizardText(m, "Guided custom workflow")
	wizardPress(m, tea.KeyTab)
	wizardText(m, "feat")
	wizardPress(m, tea.KeyTab)
	wizardText(m, "Be precise.")
	wizardPress(m, tea.KeyCtrlN)
	if w.stage != 1 || len(w.files) == 0 {
		t.Fatal(w.stage, w.files, m.status)
	}
	selected := []string{}
	for index, file := range w.files {
		if len(selected) == 0 && strings.HasPrefix(file, "components/") || len(selected) == 1 && strings.HasPrefix(file, "res/") {
			w.cursor = index
			wizardPress(m, tea.KeySpace)
			selected = append(selected, file)
		}
	}
	if len(selected) != 2 {
		t.Fatal(selected)
	}
	wizardPress(m, tea.KeyCtrlN)
	wizardText(m, "Implement carefully")
	wizardPress(m, tea.KeyCtrlA)
	wizardText(m, "Investigate")
	wizardPress(m, tea.KeyTab)
	wizardText(m, "Read the project\nIdentify the issue")
	wizardPress(m, tea.KeyTab)
	wizardText(m, "Confirm the cause")
	wizardPress(m, tea.KeyCtrlA)
	wizardText(m, "Fix")
	wizardPress(m, tea.KeyTab)
	wizardText(m, "Apply the change")
	wizardPress(m, tea.KeyTab)
	wizardText(m, "Tests pass")
	wizardPress(m, tea.KeyCtrlN)
	if w.stage != 3 || !strings.Contains(m.preview.View(), "Guided custom workflow") {
		t.Fatal(w.stage, m.status, m.preview.View())
	}
	wizardPress(m, tea.KeyCtrlS)
	if !strings.Contains(m.status, "Skill saved") {
		t.Fatal(m.status)
	}
	catalog := skills.Catalog{Source: skills.Overlay{User: os.DirFS(filepath.Join(m.options.Home, ".ashley")), Base: m.options.Catalog.Source}}
	definition, err := catalog.Load("guided")
	if err != nil {
		t.Fatal(err)
	}
	if definition.Preamble != "Be precise." || definition.Workflow.Name != "Implement carefully" || len(definition.Workflow.Steps) != 2 {
		t.Fatalf("%+v", definition)
	}
	if len(definition.Workflow.Steps[0].Instructions) != 2 || definition.Workflow.Steps[1].Validation != "Tests pass" {
		t.Fatal(definition.Workflow)
	}
	result, err := catalog.Assemble("guided", nil)
	if err != nil || !strings.Contains(result.Content, "Confirm the cause") {
		t.Fatal(result, err)
	}
	if _, err := os.Stat(filepath.Join(m.options.Home, ".ashley", "skills", "guided.jsonc")); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(m.options.Home, ".ashley", "skills", "guided.jsonc"))
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if saved["extends"] != "feat" || saved["components+"] == nil || saved["resources+"] == nil || saved["components"] != nil {
		t.Fatal(saved)
	}
	wizardPress(m, tea.KeyCtrlS)
	if !strings.Contains(m.status, "existing files are preserved") {
		t.Fatal(m.status)
	}
}

func TestGuidedCreatorBackMouseResizeAndAdvancedEditor(t *testing.T) {
	m := newModel(t)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.open("create")
	w := m.wizard
	wizardText(m, "mouse")
	m.Update(tea.MouseMsg{X: 5, Y: 15, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if w.field != 1 {
		t.Fatal(w.field)
	}
	wizardText(m, "Mouse creation")
	wizardPress(m, tea.KeyCtrlN)
	m.Update(tea.MouseMsg{X: 2, Y: 5, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if !w.selected[w.files[0]] {
		t.Fatal("mouse did not toggle resource")
	}
	m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	if w.cursor != 1 {
		t.Fatal(w.cursor)
	}
	wizardPress(m, tea.KeyCtrlN)
	wizardPress(m, tea.KeyCtrlA)
	wizardText(m, "First")
	wizardPress(m, tea.KeyCtrlA)
	wizardText(m, "Second")
	wizardPress(m, tea.KeyCtrlLeft)
	if w.input.Value() != "First" {
		t.Fatal(w.input.Value())
	}
	wizardPress(m, tea.KeyCtrlD)
	if len(w.steps) != 1 || w.steps[0].Name != "Second" {
		t.Fatal(w.steps)
	}
	wizardPress(m, tea.KeyEsc)
	if w.stage != 1 {
		t.Fatal(w.stage)
	}
	wizardPress(m, tea.KeyEsc)
	if w.basics[1] != "Mouse creation" {
		t.Fatal(w.basics)
	}
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 22})
	wizardPress(m, tea.KeyCtrlE)
	if m.wizard != nil || !strings.Contains(m.editor.Value(), "Second") {
		t.Fatal(m.editor.Value())
	}
	m.editor.SetValue(`{"name":"advanced","description":"advanced definition"}`)
	wizardPress(m, tea.KeyCtrlS)
	if !strings.Contains(m.status, "Skill saved") {
		t.Fatal(m.status)
	}
}
