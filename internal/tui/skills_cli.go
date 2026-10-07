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
	{"Community bundle", "Set up Ashley skills and optionally add all Emil Kowalski skills plus find-skills. Choose agents or reuse Ashley's installed agents; confirm the bundle in the terminal.", "bundle"},
}

func (m *Model) skillsPlaceholder() string {
	if skillsActions[m.cursor].command == "add" {
		return "Repository or URL, e.g. emilkowalski/skills"
	}
	return "Optional search term, e.g. animations"
}

func (m *Model) skillsDetail() string {
	action := skillsActions[m.cursor]
	return fmt.Sprintf("%s\n\n%s\n\nThe terminal opens for this action. Missing dependencies require consent.\n\nPress Enter to continue; return here after the command finishes.", action.label, action.description)
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
	case "bundle":
		args = []string{"__tui-command", "install", "--skills-only"}
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
