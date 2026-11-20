package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

var skillsActions = []struct{ label, description, command string }{
	{"Your skills", "Browse your Ashley skills, inspect a workflow, and use it in a new conversation.", "browse"},
	{"Create a skill", "Build a reusable workflow with the guided skill creator.", "create"},
	{"Sync skills", "Generate and install Ashley skills for your detected coding agents.", "sync"},
	{"Find skills", "Search the Skills directory. Enter an optional search term below, or open the native browser.", "find"},
	{"Add skills", "Install globally from a repository or URL. Enter a source below; the native installer lets you choose skills and agents.", "add"},
	{"Installed skills", "List globally installed skills and their agents.", "list"},
	{"Remove skills", "Choose globally installed skills to remove. The native CLI asks for confirmation.", "remove"},
	{"Check updates", "Check installed skills for available updates.", "check"},
	{"Update skills", "Update installed skills using the native CLI.", "update"},
	{"Community bundle", "Choose which community bundles Ashley installs and manages.", "bundle"},
}

func (m *Model) skillsPlaceholder() string {
	if skillsActions[m.cursor].command == "add" {
		return m.tr("Repository or URL, e.g. emilkowalski/skills")
	}
	return m.tr("Optional search term, e.g. animations")
}

func (m *Model) skillsDetail() string {
	action := skillsActions[m.cursor]
	return fmt.Sprintf("%s\n\n%s\n\n%s", m.tr(action.label), m.tr(action.description), m.tr("Press Enter to continue."))
}

func (m *Model) activateSkills() tea.Cmd {
	action := skillsActions[m.cursor].command
	switch action {
	case "browse":
		m.open("vibe")
		return nil
	case "create":
		m.open("create")
		return nil
	case "bundle":
		return m.openCommunity()
	case "sync":
		m.open("sync")
		return m.startOperation("sync")
	}
	if (action == "find" || action == "add") && m.focus != "skills-input" {
		m.focus = "skills-input"
		m.question.SetValue("")
		return m.question.Focus()
	}
	value := strings.TrimSpace(m.question.Value())
	if action == "add" && value == "" {
		m.status = "Enter a repository or URL to install."
		return nil
	}
	// Keep a source/query as one argument, never interpret it as shell text or flags.
	if (action == "add" || action == "find") && strings.HasPrefix(value, "-") {
		m.status = "Enter a source or search term, not a CLI flag."
		return nil
	}
	args := []string{"__tui-command", "skills", action}
	switch action {
	case "find", "add":
		if value != "" {
			args = append(args, value)
		}
	case "list", "remove":
		args = append(args, "--global")
	}
	if action == "add" {
		args = append(args, "--global")
	}
	m.focus = ""
	m.question.Blur()
	return m.execute(args...)
}

func (m *Model) skillsHasInput() bool {
	return m.cursor >= 0 && m.cursor < len(skillsActions) && (skillsActions[m.cursor].command == "find" || skillsActions[m.cursor].command == "add")
}
