package tui

import (
	"fmt"
	"strings"

	"github.com/LBYPatrick/ashley/internal/agents"
	"github.com/LBYPatrick/ashley/internal/config"
	"github.com/charmbracelet/lipgloss"
)

// View renders the current screen with the user's saved palette.
func (m *Model) View() string {
	primary := "#4A9EFF"
	accent := primary
	for _, p := range config.Presets() {
		if p.Key == m.theme.Preset {
			primary = p.Primary
			accent = p.Accent
		}
	}
	fg, bg := "#DCE4EE", "#111820"
	if m.theme.Mode == "light" {
		fg, bg = "#182230", "#F4F7FA"
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(primary))
	selected := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(accent))
	var body strings.Builder
	fmt.Fprintln(&body, title.Render("Ashley  /  "+strings.Title(m.screen)))
	fmt.Fprintf(&body, "%s · %s\n\n", agents.Get(m.agent).Label, m.status)
	if m.screen == "create" {
		if m.wizard != nil {
			fmt.Fprint(&body, m.wizardView())
		} else {
			fmt.Fprintln(&body, "Create skill — edit the definition below. Extends, components, resources, workflow, and checklist are supported.")
			fmt.Fprintln(&body, m.editor.View())
			fmt.Fprintln(&body, "Ctrl+P preview · Ctrl+S save · Esc cancel")
		}
	} else if m.screen == "create-preview" {
		fmt.Fprintln(&body, m.preview.View())
		fmt.Fprintln(&body, "PgUp/PgDn scroll · Esc edit")
	} else if m.screen == "settings" {
		if m.firstRun {
			fmt.Fprint(&body, "Welcome to Ashley — choose an agent and appearance.\n\n")
		}
		rows := settingsRows()
		for r, row := range rows {
			for c, label := range row {
				mark := "  "
				if label == m.agent || label == m.theme.Mode || label == m.theme.Preset {
					mark = "✓ "
				}
				text := mark + label
				if r == m.settingsRow && c == m.settingsColumn {
					text = selected.Render("[" + text + "]")
				}
				fmt.Fprint(&body, lipgloss.NewStyle().Width(16).Render(text))
			}
			fmt.Fprint(&body, "\n\n")
		}
		fmt.Fprintln(&body, "Arrows navigate · Enter selects · Esc saves and closes")
	} else if m.screen == "stats" {
		fmt.Fprintf(&body, "Total invocations: %d\n\nTop Skills\n", m.stats.Total)
		for _, v := range m.stats.TopSkills {
			fmt.Fprintf(&body, "  %-16s %5d %s\n", v.Name, v.Count, strings.Repeat("█", min(v.Count, 30)))
		}
		fmt.Fprintln(&body, "\nBy Agent")
		for _, v := range m.stats.ByAgent {
			fmt.Fprintf(&body, "  %-16s %5d %s\n", agents.Get(v.Name).Label, v.Count, strings.Repeat("█", min(v.Count, 30)))
		}
		fmt.Fprintln(&body, "\nR refresh · Esc back")
	} else if m.screen == "log" {
		fmt.Fprintln(&body, m.preview.View())
		fmt.Fprintln(&body, "PgUp/PgDn scroll · Esc back")
	} else {
		var labels []string
		switch m.screen {
		case "hub":
			labels = hub
		case "vibe":
			labels = m.names
			fmt.Fprintln(&body, m.filter.View())
		case "sessions":
			for _, s := range m.sessionRows {
				labels = append(labels, fmt.Sprintf("%s  %-12s %s", s.ID, s.Skill, s.Agent))
			}
		case "history":
			fmt.Fprintln(&body, m.filter.View())
			for _, v := range m.historyRows {
				labels = append(labels, fmt.Sprintf("%s %d %-12s %s", v.OutcomeIcon(), v.ID, v.Skill, v.TimeDisplay()))
			}
		}
		var list strings.Builder
		visible := max(3, m.height-12)
		start := max(0, m.cursor-visible+1)
		for index := start; index < min(len(labels), start+visible); index++ {
			label := "  " + labels[index]
			if index == m.cursor {
				label = selected.Render("› " + labels[index])
			}
			fmt.Fprintln(&list, label)
		}
		if len(labels) == 0 {
			fmt.Fprintln(&list, "No entries.")
		}
		fmt.Fprintln(&body, lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().Width(max(20, m.width/3)).Render(list.String()), m.preview.View()))
		switch m.screen {
		case "vibe":
			fmt.Fprint(&body, "Mode: ")
			for i, mode := range modes {
				label := " " + mode + " "
				if i == m.mode {
					label = selected.Render("[" + mode + "]")
				}
				fmt.Fprint(&body, label+" ")
			}
			fmt.Fprintf(&body, "\n%s\n", m.question.View())
			fmt.Fprintln(&body, "↑↓ select · / filter · Tab question · P copy prompt · Enter run · Esc back")
		case "sessions":
			fmt.Fprintln(&body, "Enter attach · C copy ID · L log · S sort · K kill · X kill all · D delete · R refresh · k cleanup · Esc back")
		case "history":
			fmt.Fprintf(&body, "Entries %d–%d of %d\n", min(m.offset+1, m.total), min(m.offset+20, m.total), m.total)
			fmt.Fprintln(&body, "/ search · D delete · N next · P previous · R refresh · Esc back")
		default:
			fmt.Fprintln(&body, "↑↓ select · Enter open · Q quit")
		}
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(fg)).Background(lipgloss.Color(bg)).Width(max(20, m.width)).Render(body.String())
}
