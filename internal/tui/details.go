package tui

import (
	"fmt"
	"io/fs"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/LBYPatrick/ashley/internal/agents"
	"github.com/LBYPatrick/ashley/internal/history"
	"github.com/titanous/json5"
)

var featureTitles = []string{"Vibe · Skill Browser", "Sessions", "History", "Sync", "Create skill", "Analytics", "Settings"}

var featureDescriptions = []string{
	"Browse skills, preview workflows, and launch your coding agent with a skill prompt.",
	"Manage background agent sessions. Attach, view logs, or kill running sessions.",
	"Browse your full invocation history. Search, filter, prune, or clear past runs.",
	"Generate skills, then install them for every detected coding agent.",
	"Build a new skill interactively with a step-by-step wizard.",
	"See which skills and coding agents you use most.",
	"Choose the default coding agent, light or dark mode, and a colour preset.",
}

func (m *Model) detailText() string {
	switch m.screen {
	case "hub":
		return featureTitles[m.cursor] + "\n\n" + featureDescriptions[m.cursor] + "\n\nPress Enter to open"
	case "vibe":
		if len(m.names) == 0 {
			return "No skills found."
		}
		name := m.names[m.cursor]
		// The original browser summarizes the authored definition, not expanded inheritance.
		data, err := fs.ReadFile(m.options.Catalog.Source, "skills/"+name+".jsonc")
		if err != nil {
			return err.Error()
		}
		var d struct {
			Name, Description, Extends string
			Components, Resources      []string
			ComponentsPlus             []string `json:"components+"`
			ResourcesPlus              []string `json:"resources+"`
			Workflow                   struct{ Steps []struct{ Name string } }
		}
		if err := json5.Unmarshal(data, &d); err != nil {
			return err.Error()
		}
		if d.Components == nil {
			d.Components = d.ComponentsPlus
		}
		if d.Resources == nil {
			d.Resources = d.ResourcesPlus
		}
		description := d.Description
		if description == "" {
			description = "No description provided."
		}
		text := "▸ " + d.Name + "\n" + description + "\n\nOverview\n"
		if d.Extends != "" {
			text += fmt.Sprintf("  %-11s%s\n", "Extends", d.Extends)
		}
		text += fmt.Sprintf("  %-11s%d\n  %-11s%d\n  %-11s%d", "Components", len(d.Components), "Resources", len(d.Resources), "Steps", len(d.Workflow.Steps))
		if len(d.Workflow.Steps) > 0 {
			text += "\n\nWorkflow"
			for index, step := range d.Workflow.Steps {
				text += fmt.Sprintf("\n  %2d  %s", index+1, step.Name)
			}
		}
		return text + "\n\nType a question below and press Enter to run · p to copy the prompt"
	case "sessions":
		if len(m.sessionRows) == 0 {
			return "No sessions found.\n\nStart one with: ash run --detached <skill> <question>"
		}
		s := m.sessionRows[m.cursor]
		state := "EXITED"
		if m.sessionAlive[s.ID] {
			state = "RUNNING"
		}
		question := s.Question
		if question == "" {
			question = "(no question)"
		}
		started := s.StartedAt
		if len(started) > 19 {
			started = started[:19]
		}
		started = strings.ReplaceAll(started, "T", " ")
		text := fmt.Sprintf("Session %s\n\nStatus:     %s\nSkill:      %s\nQuestion:   %s\nStarted:    %s UTC\nElapsed:    %s\nDirectory:  %s", s.ID, state, s.Skill, question, started, s.Elapsed(time.Now()), s.CWD)
		text += "\nAgent:      " + agents.Get(s.Agent).Label + "\nPermission: " + s.PermissionMode
		text += "\ntmux:       " + s.TmuxSession
		text += "\nLog:        " + s.LogFile
		return text
	case "history":
		if len(m.historyRows) == 0 {
			if m.filter.Value() != "" {
				return fmt.Sprintf("No results for %q", m.filter.Value())
			}
			if m.options.Screen == "history" {
				return "No invocation history yet.\n\nRun a skill with: ash run <skill> <question>"
			}
			return "No history yet.\n\nRun a skill with: ash run <skill> <question>"
		}
		v := m.historyRows[m.cursor]
		detached := "No"
		if v.Detached {
			detached = "Yes"
			if v.SessionID != "" {
				detached += " (session: " + v.SessionID + ")"
			}
		}
		question := v.Question
		if question == "" {
			question = "(none)"
		}
		dbpath := history.Path(m.options.Home, runtime.GOOS, os.Getenv("XDG_DATA_HOME"))
		size := "0 B"
		if info, err := os.Stat(dbpath); err == nil {
			size = fmt.Sprintf("%.1f KB", float64(info.Size())/1024)
		}
		return fmt.Sprintf("Invocation #%d\n\nTime:       %s UTC\nSkill:      %s\nAgent:      %s\nQuestion:   %s\nDirectory:  %s\nPermission: %s\nDetached:   %s\n\nDatabase: %s (%s)", v.ID, v.TimeDisplay(), v.Skill, agents.Get(v.AgentType).Label, question, v.CWD, v.Permission, detached, dbpath, size)
	}
	return ""
}
func (m *Model) refreshSessionDetails() {
	if m.sessionAlive == nil {
		m.sessionAlive = map[string]bool{}
		for _, s := range m.sessionRows {
			m.sessionAlive[s.ID] = m.manager().Alive(s)
		}
	}
	m.sessionLog = ""
	if len(m.sessionRows) > 0 {
		s := m.sessionRows[m.cursor]
		content, err := m.manager().Preview(s, 50, m.sessionAlive[s.ID])
		if err != nil {
			content = err.Error()
		}
		m.sessionLog = content
		if strings.TrimSpace(m.sessionLog) == "" {
			m.sessionLog = "(empty log)"
		}
	}
}
