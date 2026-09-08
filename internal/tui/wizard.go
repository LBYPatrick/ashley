package tui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/LBYPatrick/ashley/internal/skills"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

type creatorWizard struct {
	stage, field, cursor, step int
	basics                     [4]string
	workflowName, checklist    string
	steps                      []skills.Step
	files                      []string
	selected                   map[string]bool
	input                      textarea.Model
}

func (m *Model) startWizard() {
	w := &creatorWizard{selected: map[string]bool{}, checklist: "Task completed as specified.\nCode follows project conventions.\nTests pass (if configured)."}
	w.input = textarea.New()
	w.input.CharLimit = 0
	w.input.ShowLineNumbers = false
	w.input.SetWidth(max(20, m.width-8))
	w.input.SetHeight(max(3, m.height-17))
	for _, root := range []string{"components", "res"} {
		err := fs.WalkDir(m.options.Catalog.Source, root, func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() && (root != "components" || path.Ext(name) == ".md") && !strings.HasPrefix(entry.Name(), ".") {
				w.files = append(w.files, name)
			}
			return nil
		})
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			m.status = fmt.Sprintf("Could not list %s: %v", root, err)
		}
	}
	m.wizard = w
	w.loadField()
}

func (w *creatorWizard) fields() []string {
	if w.stage == 0 {
		return []string{"Name", "Description", "Extends (optional)", "Preamble"}
	}
	labels := []string{"Workflow name", "Checklist (one item per line)"}
	if len(w.steps) > 0 {
		labels = append(labels, "Step name", "Instructions (one per line)", "Validation")
	}
	return labels
}
func (w *creatorWizard) fieldValue() string {
	if w.stage == 0 {
		return w.basics[w.field]
	}
	switch w.field {
	case 0:
		return w.workflowName
	case 1:
		return w.checklist
	case 2:
		return w.steps[w.step].Name
	case 3:
		return strings.Join(w.steps[w.step].Instructions, "\n")
	case 4:
		return w.steps[w.step].Validation
	}
	return ""
}
func lines(value string) []string {
	result := []string{}
	for _, line := range strings.Split(value, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			result = append(result, line)
		}
	}
	return result
}
func (w *creatorWizard) collect() {
	value := w.input.Value()
	if w.stage == 0 {
		w.basics[w.field] = value
		return
	}
	if w.stage != 2 {
		return
	}
	switch w.field {
	case 0:
		w.workflowName = value
	case 1:
		w.checklist = value
	case 2:
		w.steps[w.step].Name = value
	case 3:
		w.steps[w.step].Instructions = lines(value)
	case 4:
		w.steps[w.step].Validation = value
	}
}
func (w *creatorWizard) loadField() {
	if w.stage != 0 && w.stage != 2 {
		w.input.Blur()
		return
	}
	w.input.SetValue(w.fieldValue())
	w.input.Focus()
}
func (m *Model) syncWizard() error {
	w := m.wizard
	w.collect()
	name := strings.TrimPrefix(strings.TrimSpace(w.basics[0]), "a-")
	if !skillName.MatchString(name) {
		return fmt.Errorf("enter a name using lowercase letters, digits, and hyphens")
	}
	if strings.TrimSpace(w.basics[1]) == "" {
		return fmt.Errorf("enter a description")
	}
	definition := map[string]any{"name": "a-" + name, "description": strings.TrimSpace(w.basics[1]), "checklist": lines(w.checklist)}
	components, resources := []string{}, []string{}
	for _, file := range w.files {
		if w.selected[file] {
			if strings.HasPrefix(file, "components/") {
				components = append(components, file)
			} else {
				resources = append(resources, file)
			}
		}
	}
	if extends := strings.TrimSpace(w.basics[2]); extends != "" {
		definition["extends"] = strings.TrimPrefix(extends, "a-")
		if len(components) > 0 {
			definition["components+"] = components
		}
		if len(resources) > 0 {
			definition["resources+"] = resources
		}
	} else {
		if len(components) == 0 {
			components = []string{"components/guidelines-coding.md", "components/quality-assurance.md"}
		}
		definition["components"], definition["resources"] = components, resources
	}
	if w.basics[3] != "" {
		definition["preamble"] = w.basics[3]
	}
	if len(w.steps) > 0 {
		for index, step := range w.steps {
			if strings.TrimSpace(step.Name) == "" {
				return fmt.Errorf("enter a name for workflow step %d", index+1)
			}
		}
		workflowName := strings.TrimSpace(w.workflowName)
		if workflowName == "" {
			workflowName = "Run " + name
		}
		definition["workflow"] = skills.Workflow{Name: workflowName, Steps: w.steps}
	}
	data, err := json.MarshalIndent(definition, "", "  ")
	if err != nil {
		return err
	}
	m.editor.SetValue(string(data))
	return nil
}
func (m *Model) wizardPreview() bool {
	if err := m.syncWizard(); err != nil {
		m.status = err.Error()
		return false
	}
	_, _, content, err := m.createdDefinition()
	if err != nil {
		m.status = err.Error()
		return false
	}
	m.preview.Width = max(20, m.width-6)
	m.logContent = content
	m.preview.SetContent(ansi.Wrap(content, m.readingRect().w, ""))
	m.preview.GotoTop()
	return true
}
func (m *Model) wizardKey(msg tea.KeyMsg) tea.Cmd {
	w := m.wizard
	switch msg.String() {
	case "ctrl+e":
		if err := m.syncWizard(); err != nil {
			m.status = err.Error()
			return nil
		}
		m.wizard = nil
		return m.editor.Focus()
	case "ctrl+s":
		if m.wizardPreview() {
			m.saveCreated()
		}
		return nil
	case "ctrl+n", "ctrl+p":
		w.collect()
		if w.stage == 0 && !m.wizardPreview() {
			return nil
		}
		if w.stage == 2 && !m.wizardPreview() {
			return nil
		}
		w.stage = min(3, w.stage+1)
		w.field = 0
		w.loadField()
		return nil
	case "esc", "ctrl+b":
		w.collect()
		if w.stage == 0 {
			m.wizard = nil
			m.open("hub")
		} else {
			w.stage--
			w.field = 0
			w.loadField()
		}
		return nil
	case "tab", "shift+tab":
		if w.stage == 0 || w.stage == 2 {
			w.collect()
			delta := 1
			if msg.String() == "shift+tab" {
				delta = -1
			}
			w.field = (w.field + delta + len(w.fields())) % len(w.fields())
			w.loadField()
		}
		return nil
	}
	if w.stage == 1 {
		switch msg.String() {
		case "up", "k":
			w.cursor = max(0, w.cursor-1)
		case "down", "j":
			w.cursor = min(max(0, len(w.files)-1), w.cursor+1)
		case " ", "enter":
			if len(w.files) > 0 {
				file := w.files[w.cursor]
				w.selected[file] = !w.selected[file]
			}
		}
		return nil
	}
	if w.stage == 2 {
		switch msg.String() {
		case "ctrl+a":
			w.collect()
			w.steps = append(w.steps, skills.Step{})
			w.step = len(w.steps) - 1
			w.field = 2
			w.loadField()
			return nil
		case "ctrl+d":
			if len(w.steps) > 0 {
				w.steps = append(w.steps[:w.step], w.steps[w.step+1:]...)
				w.step = max(0, w.step-1)
				w.field = 0
				w.loadField()
			}
			return nil
		case "ctrl+left", "ctrl+right":
			w.collect()
			delta := 1
			if msg.String() == "ctrl+left" {
				delta = -1
			}
			w.step = max(0, min(len(w.steps)-1, w.step+delta))
			w.loadField()
			return nil
		}
	}
	var cmd tea.Cmd
	if w.stage == 3 {
		m.preview, cmd = m.preview.Update(msg)
	} else {
		w.input, cmd = w.input.Update(msg)
	}
	return cmd
}
func (m *Model) wizardView() string {
	w := m.wizard
	var body strings.Builder
	stages := []string{"Basics", "Components & Resources", "Workflow", "Preview & Save"}
	fmt.Fprintf(&body, "Step %d/4 — %s\n", w.stage+1, stages[w.stage])
	if w.stage == 1 {
		fmt.Fprintln(&body, "Space toggles · no selections uses defaults or inherited files")
		visible := max(3, m.height-10)
		start := max(0, w.cursor-visible+1)
		for index := start; index < min(len(w.files), start+visible); index++ {
			mark, cursor := " ", " "
			if w.selected[w.files[index]] {
				mark = "x"
			}
			if index == w.cursor {
				cursor = "›"
			}
			fmt.Fprintf(&body, "%s [%s] %s\n", cursor, mark, w.files[index])
		}
	} else if w.stage == 3 {
		fmt.Fprintln(&body, m.preview.View())
	} else {
		for index, label := range w.fields() {
			marker := "  "
			if index == w.field {
				marker = "› "
			}
			fmt.Fprintln(&body, marker+label)
		}
		if w.stage == 0 {
			names, _ := m.options.Catalog.Names()
			fmt.Fprintln(&body, "Available parents: "+strings.Join(names, ", "))
		} else {
			fmt.Fprintf(&body, "Step %d/%d · Ctrl+A add · Ctrl+D delete · Ctrl+←/→ switch step\n", min(w.step+1, len(w.steps)), len(w.steps))
		}
		fmt.Fprintln(&body, w.input.View())
	}
	fmt.Fprintln(&body, "Tab/Shift+Tab field · Ctrl+N next · Esc back · Ctrl+S save · Ctrl+E JSON")
	return body.String()
}
func (m *Model) wizardMouse(msg tea.MouseMsg) tea.Cmd {
	w := m.wizard
	if w.stage == 1 {
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			return m.wizardKey(tea.KeyMsg{Type: tea.KeyUp})
		case tea.MouseButtonWheelDown:
			return m.wizardKey(tea.KeyMsg{Type: tea.KeyDown})
		case tea.MouseButtonLeft:
			visible := max(3, m.height-10)
			start := max(0, w.cursor-visible+1)
			index := start + msg.Y - 5
			if msg.Action == tea.MouseActionPress && msg.Y >= 5 && index < len(w.files) && index < start+visible {
				w.cursor = index
				file := w.files[index]
				w.selected[file] = !w.selected[file]
			}
		}
		return nil
	}
	if w.stage == 3 {
		var cmd tea.Cmd
		m.preview, cmd = m.preview.Update(msg)
		return cmd
	}
	if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress && msg.Y >= 4 && msg.Y < 4+len(w.fields()) {
		w.collect()
		w.field = msg.Y - 4
		w.loadField()
		return nil
	}
	var cmd tea.Cmd
	w.input, cmd = w.input.Update(msg)
	return cmd
}
