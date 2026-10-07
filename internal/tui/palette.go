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
	{"Home", "Start or continue work"}, {"Activity", "Open running sessions and their logs"}, {"History", "Find and resume past conversations"}, {"Library", "Browse, create, sync, and manage skills.sh community skills"}, {"Choose a skill", "Browse skills and start a run"}, {"Sync", "Generate skills and install them for detected agents"}, {"Create", "Create a custom skill"}, {"Usage", "Explore skill and agent analytics"}, {"Settings", "Change your default agent, theme, and appearance"}, {"Keys", "Show all keyboard shortcuts"}, {"Maximize", "Expand the current list"}, {"Screenshot", "Save this screen as an SVG"}, {"Quit", "Exit Ashley"},
}

func (m *Model) paletteItems() [][2]string {
	var items [][2]string
	commands := append([][2]string{}, paletteCommands...)
	if m.screen == "sessions" {
		commands = append(commands, [2]string{"Clean finished sessions", "Remove finished session records and logs"})
	}
	for _, item := range commands {
		if strings.Contains(strings.ToLower(item[0]+" "+item[1]), strings.ToLower(m.paletteQuery)) {
			items = append(items, item)
		}
	}
	return items
}
func (m *Model) paletteBounds() rect {
	width := min(76, max(12, m.width-6))
	return rect{(max(20, m.width) - width) / 2, 2, width, min(max(6, m.height-4), 18)}
}
func (m *Model) paletteView(f *frame, a appearance) {
	r := m.paletteBounds()
	items := m.paletteItems()
	f.fill(r, a.panel)
	f.box(r, a.border)
	f.input(rect{r.x + 2, r.y + 1, r.w - 4, 3}, m.paletteQuery, "Search for commands…", true, a)
	visible := max(1, r.h-7)
	start := max(0, m.paletteCursor-visible+1)
	for i := start; i < min(len(items), start+visible); i++ {
		style := a.panel
		marker := "  "
		if i == m.paletteCursor {
			style = a.selected
			marker = "› "
		}
		label := ansi.Truncate(marker+items[i][0], r.w-6, "")
		f.put(r.x+3, r.y+5+i-start, style.Render(label+strings.Repeat(" ", max(0, r.w-6-ansi.StringWidth(label)))))
	}
	if len(items) == 0 {
		f.put(r.x+3, r.y+5, a.muted.Render("No matching commands"))
	} else {
		f.put(r.x+3, r.y+r.h-2, a.muted.Render(ansi.Truncate(items[m.paletteCursor][1], r.w-6, "…")))
	}
}
func (m *Model) paletteKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case " ":
		m.paletteQuery += " "
		m.paletteCursor = 0
	case "esc", "ctrl+p":
		m.paletteOpen = false
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
			return nil
		}
		choice := items[m.paletteCursor][0]
		m.paletteOpen = false
		switch choice {
		case "Clean finished sessions":
			m.requestConfirmation("cleanup")
		case "Quit":
			return m.quit()
		case "New run", "Library", "Home", "Choose a skill", "Activity", "History", "Sync", "Create", "Usage", "Settings":
			screens := map[string]string{"New run": "compose", "Library": "skills.sh", "Home": "hub", "Choose a skill": "vibe", "Activity": "sessions", "History": "history", "Sync": "sync", "Create": "create", "Usage": "stats", "Settings": "settings"}
			m.open(screens[choice])
			if choice == "New run" {
				return m.composer.Focus()
			}
			if choice == "Sync" {
				return m.startOperation("sync")
			}
		case "Maximize":
			if m.width >= 76 && (m.screen == "vibe" || m.screen == "sessions" || m.screen == "history") {
				m.maximized = !m.maximized
				m.updatePreview()
			}
		case "Keys":
			if m.screen != "help" {
				m.helpReturn = m.screen
				m.helpPreview = m.preview
				m.helpLogContent = m.logContent
			}
			m.logContent = "Keyboard shortcuts\n\nCtrl+1 Home · Ctrl+2 New run · Ctrl+3 Activity · Ctrl+4 Library · Ctrl+5 Settings\nCtrl+P commands · ? help · Esc back · Ctrl+C quit\n\nNew run\nWrite a multiline task. Ctrl+R starts the conversation.\nCtrl+S chooses a skill; Ctrl+D removes it.\nTab moves through task, agent, permissions, and Start.\nEnter changes an option; Enter in the task inserts a newline.\n\nSkill browser\n/ searches · Enter selects · PgUp/PgDn previews\nV shows details on narrow terminals · I shows metadata\n\nActivity\nT switches Sessions and History · Enter attaches or resumes\nI toggles the inspector · L opens a session log\nR refreshes · S sorts sessions · C copies the session ID\nK stops one session · X stops all · D deletes a record\nN/P change history pages · / searches history\n\nLibrary\nBrowse your skills, create a workflow, or sync installed skills.\nChoose Find or Add to discover community skills.\n\nCreator\nCtrl+N next · Ctrl+S save · Ctrl+E JSON\nWorkflow: Ctrl+A add step · Ctrl+D delete step\n\nSettings\nArrows or Tab/Shift+Tab move · Enter selects\nChanges save automatically. Esc returns Home."
			m.sizeLogPreview()
			m.preview.GotoTop()
			m.screen = "help"
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
