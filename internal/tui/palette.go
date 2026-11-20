package tui

import (
	"fmt"
	"html"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

var paletteCommands = [][2]string{
	{"New run", "Write a task and choose an agent"},
	{"About Ashley", "Read the version, project, and MIT license information"},
	{"Home", "Start or continue work"}, {"Activity", "Find running and finished sessions and their logs"}, {"Library", "Browse, create, sync, and manage skills.sh community skills"}, {"Choose a skill", "Browse skills and start a run"}, {"Sync", "Generate skills and install them for detected agents"}, {"Create", "Create a custom skill"}, {"Usage", "Explore skill and agent analytics"}, {"Settings", "Change your default agent, theme, and appearance"}, {"Keys", "Show all keyboard shortcuts"}, {"Maximize", "Expand the current list"}, {"Screenshot", "Save this screen as an SVG"}, {"Quit", "Exit Ashley"},
}

var sessionPaletteCommands = [][2]string{
	{"Kill all sessions", "Stop all Ashley sessions and keep their logs"},
	{"Clean finished sessions", "Remove finished session records and keep their logs"},
}

func (m *Model) openSessionActions() {
	m.paletteOpen, m.paletteSessionActions = true, true
	m.paletteQuery, m.paletteCursor = "", 0
}

func (m *Model) paletteItems() [][2]string {
	var items [][2]string
	commands := append([][2]string{}, paletteCommands...)
	if m.paletteSessionActions {
		commands = sessionPaletteCommands
	} else if m.screen == "sessions" || m.screen == "activity" {
		commands = append(commands, sessionPaletteCommands...)
	}
	for _, item := range commands {
		if strings.Contains(strings.ToLower(item[0]+" "+item[1]+" "+m.tr(item[0])+" "+m.tr(item[1])), strings.ToLower(m.paletteQuery)) {
			items = append(items, item)
		}
	}
	return items
}
func (m *Model) paletteBounds() rect {
	width := min(76, max(12, m.width-6))
	height := min(max(6, m.height-4), 18)
	if m.paletteSessionActions {
		width = min(width, 54)
		height = min(height, 9)
	}
	return rect{(max(20, m.width) - width) / 2, 2, width, height}
}
func (m *Model) paletteView(f *frame, a appearance) {
	placeholder := "Search for commands…"
	if m.paletteSessionActions {
		body, selected := confirmationStyles(a)
		a.base, a.panel, a.border, a.muted = body, body, body, body
		a.title, a.selected = body.Bold(true), selected
		placeholder = "Session actions…"
	}
	r := m.paletteBounds()
	items := m.paletteItems()
	f.fill(r, a.panel)
	f.box(r, a.border)
	f.input(rect{r.x + 2, r.y + 1, r.w - 4, 3}, m.paletteQuery, m.tr(placeholder), true, a)
	visible := max(1, r.h-7)
	start := max(0, m.paletteCursor-visible+1)
	for i := start; i < min(len(items), start+visible); i++ {
		style := a.panel
		marker := "  "
		if i == m.paletteCursor {
			style = a.selected
			marker = "› "
		}
		label := ansi.Truncate(marker+m.tr(items[i][0]), r.w-6, "")
		f.put(r.x+3, r.y+5+i-start, style.Render(label+strings.Repeat(" ", max(0, r.w-6-ansi.StringWidth(label)))))
	}
	if len(items) == 0 {
		f.put(r.x+3, r.y+5, a.muted.Render(m.tr("No matching commands")))
	} else {
		f.put(r.x+3, r.y+r.h-2, a.muted.Render(ansi.Truncate(m.tr(items[m.paletteCursor][1]), r.w-6, "…")))
	}
}
func (m *Model) paletteKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case " ":
		m.paletteQuery += " "
		m.paletteCursor = 0
	case "esc", "ctrl+p":
		m.paletteOpen = false
		m.paletteSessionActions = false
	case "ctrl+c":
		return m.quit()
	case "up":
		m.paletteCursor = max(0, m.paletteCursor-1)
	case "down":
		m.paletteCursor = min(max(0, len(m.paletteItems())-1), m.paletteCursor+1)
	case "backspace":
		r := []rune(m.paletteQuery)
		if len(r) > 0 {
			m.paletteQuery = string(r[:len(r)-1])
		}
		m.paletteCursor = 0
	case "enter":
		items := m.paletteItems()
		if len(items) == 0 {
			if m.paletteSessionActions {
				m.paletteOpen, m.paletteSessionActions = false, false
			}
			return nil
		}
		choice := items[m.paletteCursor][0]
		m.paletteOpen = false
		m.paletteSessionActions = false
		switch choice {
		case "About Ashley":
			m.openAbout()
		case "Kill all sessions":
			m.requestConfirmation("X")
		case "Clean finished sessions":
			m.requestConfirmation("cleanup")
		case "Quit":
			return m.quit()
		case "New run", "Library", "Home", "Choose a skill", "Activity", "Sync", "Create", "Usage", "Settings":
			screens := map[string]string{"New run": "compose", "Library": "skills.sh", "Home": "hub", "Choose a skill": "vibe", "Activity": "activity", "Sync": "sync", "Create": "create", "Usage": "stats", "Settings": "settings"}
			m.open(screens[choice])
			if choice == "New run" {
				return m.composer.Focus()
			}
			if choice == "Sync" {
				return m.startOperation("sync")
			}
		case "Maximize":
			if m.width >= 76 && (m.screen == "vibe" || m.isActivity()) {
				m.maximized = !m.maximized
				m.updatePreview()
			}
		case "Keys":
			m.openShortcutHelp()
		case "Screenshot":
			name := "ashley-" + time.Now().Format("20060102-150405") + ".svg"
			text := ansi.Strip(m.View())
			a := m.appearance()
			var svg strings.Builder
			fmt.Fprintf(&svg, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d"><rect width="100%%" height="100%%" fill="%s"/><g font-family="monospace" font-size="14" fill="%s">`, m.width*9, m.height*18, a.bg, a.fg)
			for index, line := range strings.Split(text, "\n") {
				fmt.Fprintf(&svg, `<text x="0" y="%d" xml:space="preserve">%s</text>`, index*18+14, html.EscapeString(line))
			}
			svg.WriteString("</g></svg>")
			if err := os.WriteFile(name, []byte(svg.String()), 0600); err != nil {
				m.status = err.Error()
			} else {
				m.status = "Screenshot saved: " + name
			}
		}
	default:
		if msg.Type == tea.KeyRunes {
			m.paletteQuery += string(msg.Runes)
			m.paletteCursor = 0
		}
	}
	return nil
}
