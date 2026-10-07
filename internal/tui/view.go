package tui

import (
	"fmt"
	"github.com/LBYPatrick/ashley/internal/agents"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type appearance struct {
	base, border, title, muted, selected, blurred, panel lipgloss.Style
	accent, bg, fg                                       string
}

// effectiveMode uses terminal brightness for Auto. Lipgloss/termenv falls
// back to a black background when the terminal cannot report its color.
func (m *Model) effectiveMode() string {
	if m.theme.Mode == "dark" || m.theme.Mode == "light" {
		return m.theme.Mode
	}
	if lipgloss.HasDarkBackground() {
		return "dark"
	}
	return "light"
}
func (m *Model) themePalette() map[string]string {
	mode := m.effectiveMode()
	values := themeValues[mode+"-"+m.theme.Preset]
	if values == nil {
		values = themeValues["dark-blue"]
	}
	return values
}

func (m *Model) appearance() appearance {
	values := m.themePalette()
	accent := terminalColor(values["accent"])
	bg, border, panel := values["surface"], values["surface-lighten-2"], terminalColor(values["panel"])
	contrast := "#FFFFFF"
	if m.effectiveMode() == "light" {
		contrast = "#000000"
	}
	fg, muted := blendColor(contrast, bg, .87), blendColor(contrast, bg, .60)
	base := lipgloss.NewStyle().Background(lipgloss.Color(bg)).Foreground(lipgloss.Color(fg))
	return appearance{base: base, border: base.Foreground(lipgloss.Color(border)), title: base.Foreground(lipgloss.Color(accent)).Bold(true), muted: base.Foreground(lipgloss.Color(muted)), selected: base.Background(lipgloss.Color(blendColor(accent, bg, .20))).Bold(true), blurred: base.Background(lipgloss.Color(blendColor(accent, bg, .3))), panel: base.Background(lipgloss.Color(panel)), accent: accent, bg: bg, fg: fg}
}

type screenLayout struct{ left, right, list, detail, input rect }

func (m *Model) layout() screenLayout {
	w, h := max(20, m.width), max(8, m.height)
	margin := 2
	if m.screen == "hub" {
		margin = max(2, (w-132)/2)
	}
	available := w - 2*margin
	sidebar := 28
	if m.screen == "sessions" {
		sidebar = min(56, max(34, available/4))
	}
	if m.screen == "history" {
		sidebar = min(64, max(38, available/4))
	}
	sidebar = min(sidebar, max(12, available*2/5))
	bottom := h - 3
	if m.screen == "skills.sh" {
		bottom = h - 6
	}
	l := screenLayout{}
	l.left = rect{margin, 3, sidebar, max(3, bottom-3)}
	l.right = rect{margin + sidebar + 3, 3, max(1, available-sidebar-3), max(3, bottom-3)}
	l.list = rect{margin, 6, sidebar, max(1, bottom-6)}
	l.detail = rect{l.right.x, 5, l.right.w, max(1, bottom-5)}
	l.input = rect{margin, h - 5, available, 3}
	if m.screen == "vibe" || m.screen == "history" {
		l.input = rect{margin, 4, sidebar, 3}
		l.list.y = 8
		l.list.h = max(1, bottom-8)
	}
	if m.maximized {
		l.list.w = available
		l.left.w = available
		l.right = rect{}
		l.detail = rect{}
		if m.screen == "vibe" || m.screen == "history" {
			l.input.w = available
		}
	}
	if w < 76 {
		l.left.w = available
		l.list.w = available
		if m.screen == "vibe" || m.screen == "history" {
			l.input.w = available
		}
		l.right = rect{}
		l.detail = rect{}
		if m.compactDetail {
			l.right = rect{margin, 3, available, max(1, bottom-3)}
			l.detail = rect{margin, 4, available, max(1, bottom-4)}
		}
	}
	return l
}
func (m *Model) header(f *frame, a appearance) {
	f.fill(rect{0, 0, f.width, 2}, a.base)
	f.put(2, 0, a.base.Bold(true).Render(ansi.Truncate("Ashley  /  "+screenName(m.screen), max(1, f.width-18), "…")))
	f.put(max(2, f.width-15), 0, a.muted.Render("^P Commands"))
	for _, c := range m.navControls() {
		style := a.muted
		active := m.screen == c.key || (c.key == "sessions" && (m.screen == "history" || m.screen == "log")) || (c.key == "skills.sh" && (m.screen == "vibe" || m.screen == "create" || m.screen == "sync"))
		if active {
			style = a.selected
		}
		f.put(c.x, c.y, style.Render(ansi.Truncate(" "+c.label+" ", c.w, "")))
	}
}
func (m *Model) footer(f *frame, a appearance) {
	text := "enter Open  ↑↓ Move  c Continue latest  q Quit"
	if m.screen == "hub" && len(m.recent) == 0 {
		text = "enter Open  ↑↓ Move  q Quit"
	}
	switch m.screen {
	case "skills.sh":
		text = "esc Back  ↑↓ Move  enter Open / Submit"
	case "vibe":
		text = "enter Use skill  / Search  pgdn Preview  esc Back"
	case "compose":
		text = "^R Start  ^N Name  ^S Skill  tab Options  esc Home"
	case "sessions":
		text = "enter Open  t History  l Log  N Rename  i Inspector  K Stop"
		if m.options.Screen == "sessions" {
			text += "  q Quit"
		}
	case "history":
		text = "enter Resume  t Sessions  / Search  N Rename  i Inspector  d Delete"
		if m.options.Screen == "history" {
			text += "  q Quit"
		}
	case "settings":
		text = "esc Done  ↑↓←→ Move  enter Select  tab Next"
	case "stats":
		text = "esc Back  r Refresh"
	case "create":
		text = "esc Back  ^n Next  ^s Save  ^e JSON"
		if m.wizard == nil {
			text = "esc Back  ^s Save  ^r Preview"
		} else if m.wizard.stage == 3 {
			text = "esc Back  ^s Save  ^e JSON  ↑↓ Scroll"
		}
	case "create-preview":
		text = "esc Back to editor  ↑↓ Scroll"
	case "sync":
		text = "esc Back  enter Sync  r Sync again  i Set up CLI"
	case "help", "log":
		text = "esc Back  pgup/pgdown Scroll"
	}
	if m.width < 76 && (m.screen == "vibe" || m.screen == "sessions" || m.screen == "history" || m.screen == "skills.sh") {
		text = "enter Open  v Details  / Search  esc Back"
		if m.screen == "vibe" {
			text = "enter Use skill  v Details  / Search"
		}
		if m.screen == "skills.sh" {
			text = "enter Open  v Details  esc Back"
		}
	}
	if m.focus == "filter" {
		text = "Type to search  enter Results  esc Done"
	}
	if m.confirm != nil {
		text = "tab Choose  enter Accept  esc Cancel"
	}
	y := f.height - 1
	f.fill(rect{0, y, f.width, 1}, a.panel)
	f.put(2, y, a.panel.Render(ansi.Truncate(text, max(1, f.width-2), "…")))
	x := 2
	for _, binding := range strings.Split(text, "  ") {
		key := strings.Fields(binding)
		if len(key) > 0 && x+len(key[0]) < f.width-2 {
			f.put(x, y, a.panel.Foreground(lipgloss.Color(a.accent)).Render(key[0]))
		}
		x += ansi.StringWidth(binding) + 2
	}
}
func (f *frame) input(r rect, value, placeholder string, focused bool, a appearance, cursor ...int) {
	if r.w < 4 {
		return
	}
	border := a.border
	if focused {
		border = a.title
	}
	f.fill(r, a.panel)
	f.put(r.x, r.y+2, border.Render(strings.Repeat("─", r.w)))
	style := a.panel
	empty := value == ""
	if value == "" {
		value = placeholder
		style = a.muted.Background(a.panel.GetBackground())
	}
	chars := []rune(value)
	position := len(chars)
	if len(cursor) > 0 {
		position = cursor[0]
	}
	if empty {
		position = 0
	}
	position = min(max(0, position), len(chars))
	caret := ansi.StringWidth(string(chars[:position]))
	shift := 0
	if focused {
		shift = max(0, caret-max(1, r.w-6))
	}
	f.put(r.x+3, r.y+1, style.Render(ansi.Cut(value, shift, shift+max(1, r.w-5))))
	if focused {
		glyph := " "
		if position < len(chars) {
			glyph = string(chars[position])
		}
		f.put(r.x+3+caret-shift, r.y+1, a.cursor().Render(glyph))
	}
}

// View composes every page with shared chrome and bounded terminal cells.
func (m *Model) View() string {
	a := m.appearance()
	f := newFrame(max(20, m.width), max(8, m.height), a.base)
	m.header(f, a)
	switch m.screen {
	case "hub":
		m.homeView(f, a)
	case "compose":
		m.composeView(f, a)
	case "vibe", "sessions", "history", "skills.sh":
		m.browserView(f, a)
	case "sync":
		m.operationView(f, a)
	case "settings":
		m.settingsView(f, a)
	case "stats":
		m.statsView(f, a)
	case "create":
		m.creatorView(f, a)
	case "create-preview", "log", "help":
		r := m.readingRect()
		f.text(r, m.preview.View(), a.base, 0)

	}
	m.header(f, a)
	if m.status != "" { // Notifications occupy chrome, never replace a detail panel.
		text := ansi.Truncate(m.status, max(1, f.width-4), "…")
		f.put(max(1, f.width-ansi.StringWidth(text)-2), f.height-2, a.panel.Render(text))
	}
	m.footer(f, a)
	if m.paletteOpen {
		m.paletteView(f, a)
	}
	if m.confirm != nil {
		m.confirmView(f, a)
	}
	if m.rename != nil {
		m.nameView(f, a)
	}
	return f.String()
}
func (m *Model) statsView(f *frame, a appearance) {
	r := m.readingRect()
	text := m.statsText(a)
	offset := min(m.screenScroll, max(0, wrappedLines(text, r.w)-r.h))
	f.text(r, text, a.base, offset)
	f.scrollbar(rect{r.x + r.w, r.y, 1, r.h}, wrappedLines(text, r.w), offset, a)
}
func (m *Model) statsMaxScroll() int {
	r := m.readingRect()
	return max(0, wrappedLines(m.statsText(m.appearance()), r.w)-r.h)
}
func (m *Model) statsText(a appearance) string {
	text := a.title.Render("◆ Analytics") + "\n\n  " + a.muted.Render("Total invocations") + "   " + a.base.Bold(true).Render(fmt.Sprint(m.stats.Total))
	heading := a.title.Foreground(lipgloss.Color(terminalColor(m.themePalette()["accent-lighten-1"])))
	accent := m.themePalette()["accent"]
	if len(m.stats.TopSkills) == 0 {
		text += "\n\n" + a.muted.Render("No invocations recorded yet.")
	} else {
		text += "\n\n" + heading.Render("Top skills")
		peak := 0
		for _, v := range m.stats.TopSkills {
			peak = max(peak, v.Count)
		}
		shades := accentGradient(accent, len(m.stats.TopSkills))
		for i, v := range m.stats.TopSkills {
			width := max(1, int(math.RoundToEven(float64(v.Count)/float64(max(1, peak))*24)))
			text += "\n  " + a.base.Bold(true).Render(fmt.Sprintf("%-12s", v.Name)) + " " + a.base.Foreground(lipgloss.Color(shades[i])).Render(strings.Repeat("█", width)) + " " + a.muted.Render(fmt.Sprint(v.Count))
		}
	}
	if len(m.stats.ByAgent) > 0 {
		text += "\n\n" + heading.Render("By agent")
		peak := 0
		for _, v := range m.stats.ByAgent {
			peak = max(peak, v.Count)
		}
		shades := accentGradient(accent, len(m.stats.ByAgent))
		for i, v := range m.stats.ByAgent {
			width := max(1, int(math.RoundToEven(float64(v.Count)/float64(max(1, peak))*24)))
			text += "\n  " + a.base.Bold(true).Render(fmt.Sprintf("%-14s", agents.Get(v.Name).Label)) + " " + a.base.Foreground(lipgloss.Color(shades[i])).Render(strings.Repeat("█", width)) + " " + a.muted.Render(fmt.Sprint(v.Count))
		}
	}
	return text
}

func (a appearance) cursor() lipgloss.Style {
	return a.panel.Reverse(true)
}
