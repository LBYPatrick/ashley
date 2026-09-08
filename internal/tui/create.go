package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/LBYPatrick/ashley/internal/skills"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/titanous/json5"
)

var skillName = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func (m *Model) startCreator() {
	editor := textarea.New()
	editor.CharLimit = 0
	editor.SetWidth(max(30, m.width-8))
	editor.SetHeight(max(8, m.height-10))
	editor.ShowLineNumbers = true
	editor.SetValue(`{
  "name": "a-my-skill",
  "description": "What this skill does",
  "components": ["components/guidelines-coding.md", "components/quality-assurance.md"],
  "resources": [],
  "preamble": "",
  "workflow": {"name": "Run my skill", "steps": []},
  "checklist": ["Task completed as specified.", "Code follows project conventions.", "Tests pass (if configured)."]
}`)
	editor.Focus()
	m.editor = editor
	m.startWizard()
}
func (m *Model) createKey(msg tea.KeyMsg) tea.Cmd {
	if m.wizard != nil {
		return m.wizardKey(msg)
	}
	switch msg.String() {
	case "esc":
		m.open("hub")
		return nil
	case "ctrl+s":
		m.saveCreated()
		return nil
	case "ctrl+p":
		_, _, preview, err := m.createdDefinition()
		if err != nil {
			m.status = err.Error()
		} else {
			m.preview.SetContent(preview)
			m.screen = "create-preview"
		}
		return nil
	}
	var cmd tea.Cmd
	m.editor, cmd = m.editor.Update(msg)
	return cmd
}
func (m *Model) createdDefinition() (string, []byte, string, error) {
	var definition map[string]any
	if err := json5.Unmarshal([]byte(m.editor.Value()), &definition); err != nil {
		return "", nil, "", err
	}
	name, _ := definition["name"].(string)
	if len(name) > 2 && name[:2] == "a-" {
		name = name[2:]
	}
	if !skillName.MatchString(name) {
		return "", nil, "", fmt.Errorf("skill name must contain lowercase letters, digits, and hyphens")
	}
	definition["name"] = "a-" + name
	definition["output"] = "generated/a-" + name + "/SKILL.md"
	data, err := json.MarshalIndent(definition, "", "  ")
	if err != nil {
		return "", nil, "", err
	}
	catalog := skills.Catalog{Source: skills.WithDefinition(m.options.Catalog.Source, name, data)}
	result, err := catalog.Assemble(name, nil)
	return name, append(data, '\n'), result.Content, err
}
func (m *Model) saveCreated() {
	name, data, _, err := m.createdDefinition()
	if err != nil {
		m.status = err.Error()
		return
	}
	root := m.options.Root
	if root == "" {
		root = filepath.Join(m.options.Home, ".ashley")
	}
	directory := filepath.Join(root, "skills")
	if err := os.MkdirAll(directory, 0755); err != nil {
		m.status = err.Error()
		return
	}
	path := filepath.Join(directory, name+".jsonc")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		m.status = fmt.Sprintf("Could not create skill (existing files are preserved): %v", err)
		return
	}
	_, err = file.Write(data)
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(path)
		m.status = err.Error()
		return
	}
	m.status = "Skill saved: " + path
}
