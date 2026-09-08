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
	{"Home", "Open the Ashley hub"}, {"Skills", "Browse skills and start a run"}, {"Sessions", "Inspect detached runs and their logs"}, {"History", "Browse past invocations"}, {"Generate", "Rebuild prompts and review the result"}, {"Install", "Install embedded skills into your user directories"}, {"Create", "Create a custom skill"}, {"Analytics", "Explore skill and agent usage"}, {"Theme", "Open Settings and change appearance"}, {"Keys", "Show all keyboard shortcuts"}, {"Maximize", "Expand the current list"}, {"Screenshot", "Save this screen as an SVG"}, {"Quit", "Exit Ashley"},
}

func (m *Model) paletteItems() [][2]string {
	var items [][2]string
	for _, item := range paletteCommands {
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
		case "Quit":
			return m.quit()
		case "Home", "Skills", "Sessions", "History", "Generate", "Install", "Create", "Analytics", "Theme":
			screens := map[string]string{"Home": "hub", "Skills": "vibe", "Sessions": "sessions", "History": "history", "Generate": "generate", "Install": "install", "Create": "create", "Analytics": "stats", "Theme": "settings"}
			m.open(screens[choice])
			if choice == "Generate" && (m.job == nil || m.job.kind != "generate") {
				return m.startOperation("generate")
			}
		case "Maximize":
			if m.screen == "hub" || m.screen == "vibe" || m.screen == "sessions" || m.screen == "history" {
				m.maximized = !m.maximized
				m.updatePreview()
			}
		case "Keys":
			if m.screen != "help" {
				m.helpReturn = m.screen
				m.helpPreview = m.preview
				m.helpLogContent = m.logContent
			}
			m.logContent = "Keyboard shortcuts\n\nArrows select · Enter opens or runs\nTab changes focus · / searches\nPgUp/PgDn scroll details\nEsc returns · Ctrl+P opens commands\n\nVibe: M mode · P copy prompt\nSessions: C copy ID · L log · S sort\nK kill · X kill all · D delete · R refresh\nHistory: N/P pages · D delete\nCreator: Ctrl+N next · Ctrl+S save · Ctrl+E JSON\nWorkflow: Ctrl+A add step · Ctrl+D delete step\nSettings: Arrows or Tab/Shift+Tab move · Enter selects\nGenerate: R rerun · Arrows scroll results\nInstall: Enter installs skills for all detected agents · I agent CLI"
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
