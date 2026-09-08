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

var paletteCommands = [][2]string{{"Keys", "Show help for the focused widget and a summary of available keys"}, {"Maximize", "Maximize the focused widget"}, {"Quit", "Quit the application as soon as possible"}, {"Screenshot", "Save an SVG 'screenshot' of the current screen"}, {"Theme", "Change the current theme"}}

func (m *Model) paletteItems() [][2]string {
	var items [][2]string
	for _, item := range paletteCommands {
		if strings.Contains(strings.ToLower(item[0]+" "+item[1]), strings.ToLower(m.paletteQuery)) {
			items = append(items, item)
		}
	}
	return items
}
func (m *Model) paletteView(f *frame, a appearance) {
	items := m.paletteItems()
	height := min(len(f.rows)-4, 6+len(items)*2)
	f.fill(rect{0, 3, f.width, height}, a.panel)
	f.put(0, 3, a.title.Render(strings.Repeat("▔", f.width)))
	f.input(rect{4, 4, f.width - 5, 3}, m.paletteQuery, "Search for commands…", true, a)
	f.put(2, 5, a.panel.Render("🔎"))
	for index, item := range items {
		style := a.panel
		if index == m.paletteCursor {
			style = a.selected
		}
		f.put(2, 8+index*2, style.Render(ansi.Truncate(item[0], f.width-4, "")))
		f.put(2, 9+index*2, a.panel.Render(ansi.Truncate(item[1], f.width-4, "")))
	}
}
func (m *Model) paletteKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "ctrl+p":
		m.paletteOpen = false
	case "ctrl+c":
		return tea.Quit
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
			return tea.Quit
		case "Theme":
			m.open("settings")
		case "Maximize":
			m.maximized = !m.maximized
		case "Keys":
			m.preview.SetContent("Keyboard shortcuts\n\nArrows select · Enter opens or runs\nTab changes focus · / searches\nPgUp/PgDn scroll details\nEsc returns · Ctrl+P opens commands\n\nVibe: M mode · P copy prompt\nSessions: C copy ID · L log · S sort\nK kill · X kill all · D delete · R refresh\nHistory: N/P pages · D delete\nCreator: Ctrl+N next · Ctrl+S save")
			m.screen = "log"
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
