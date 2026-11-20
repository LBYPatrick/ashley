package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/LBYPatrick/ashley/internal/agents"
	"github.com/LBYPatrick/ashley/internal/config"
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
	if m.isActivity() {
		sidebar = min(52, max(34, available*42/100))
		sidebar = min(sidebar, available/2)
	} else {
		sidebar = min(sidebar, max(12, available*2/5))
	}
	bottom := m.footerTop() - 2
	if m.screen == "skills.sh" {
		bottom -= 3
	}
	l := screenLayout{}
	l.left = rect{margin, 3, sidebar, max(3, bottom-3)}
	l.right = rect{margin + sidebar + 3, 3, max(1, available-sidebar-3), max(3, bottom-3)}
	l.list = rect{margin, 6, sidebar, max(1, bottom-6)}
	l.detail = rect{l.right.x, 5, l.right.w, max(1, bottom-5)}
	l.input = rect{margin, m.footerTop() - 4, available, 3}
	if m.screen == "vibe" || m.isActivity() {
		l.input = rect{margin, 4, sidebar, 3}
		l.list.y = 8
		l.list.h = max(1, bottom-8)
	}
	if m.isActivity() {
		l.input = rect{margin, 5, sidebar, 1}
	}
	if m.maximized {
		l.list.w = available
		l.left.w = available
		l.right = rect{}
		l.detail = rect{}
		if m.screen == "vibe" || m.isActivity() {
			l.input.w = available
		}
	}
	if w < 76 {
		l.left.w = available
		l.list.w = available
		if m.screen == "vibe" || m.isActivity() {
			l.input.w = available
		}
		l.right = rect{}
		l.detail = rect{}
		if m.compactDetail {
			l.right = rect{margin, 3, available, max(1, bottom-3)}
			l.detail = rect{margin, 4, available, max(1, bottom-4)}
		}
	}
	if m.isActivity() && h < 12 {
		l.left.y = 2
		l.input = rect{l.left.x, 3, l.left.w, 1}
		l.list.y, l.list.h = 5, max(1, bottom-5)
		if l.detail.w > 0 {
			l.right.y = 2
			l.detail.y, l.detail.h = 3, max(1, bottom-3)
		}
	}
	return l
}
func (m *Model) header(f *frame, a appearance) {
	f.fill(rect{0, 0, f.width, 2}, a.base)
	title := "Ashley  /  " + m.tr(screenName(m.screen))
	titleWidth := f.width - 18
	if m.screen == "help" && f.width < 64 {
		title = m.tr("Keyboard shortcuts")
		if f.width < 36 {
			title = m.tr("Keys")
		}
		titleWidth = f.width - 4
	} else {
		f.put(max(2, f.width-15), 0, a.muted.Render(ansi.Truncate(m.localizedBinding("^P Commands"), 13, "…")))
	}
	f.put(2, 0, a.base.Bold(true).Render(ansi.Truncate(title, max(1, titleWidth), "…")))
	for _, c := range m.navControls() {
		style := a.muted
		active := m.screen == c.key || (c.key == "activity" && (m.isActivity() || m.screen == "log")) || (c.key == "skills.sh" && (m.screen == "vibe" || m.screen == "create" || m.screen == "sync" || m.screen == "community"))
		if active {
			style = a.selected
		}
		f.put(c.x, c.y, style.Render(ansi.Truncate(" "+c.label+" ", c.w, "")))
	}
}
func (m *Model) footerBindings() []string {
	text := "enter Open  ↑↓ Move  c Continue latest  q Quit"
	if m.screen == "hub" && len(m.recent) == 0 {
		text = "enter Open  ↑↓ Move  q Quit"
	}
	switch m.screen {
	case "community":
		if !m.paletteOpen && m.rename == nil && m.confirm == nil {
			return m.communityBindings()
		}
	case "skills.sh":
		text = "esc Back  ↑↓ Move  enter Open / Submit"
	case "vibe":
		text = "enter Use skill  / Search  pgdn Preview  esc Back"
	case "compose":
		text = "^R Start  ^N Name  ^S Skill  tab Options  esc Home"
	case "activity":
		text = "enter Attach/Resume  ↑↓ Move  / Search  ^F Scope  ^U Clear  n/p Pages  N Rename  l Log  c Copy ID  s Sort  r Refresh  i Metadata  K Stop  X Stop all  D Clean finished  d Delete  pgup/pgdown Preview  esc Back"
		if m.options.Screen == "activity" {
			text += "  q Quit"
		}
	case "sessions":
		text = "enter Attach/Resume  ↑↓ Move  / Search  ^F Scope  ^U Clear  t History  N Rename  l Log  c Copy ID  s Sort  r Refresh  i Metadata  K Stop  X Stop all  D Clean finished  d Delete  pgup/pgdown Preview  esc Back"
		if m.options.Screen == "sessions" {
			text += "  q Quit"
		}
	case "history":
		text = "enter Resume  ↑↓ Move  / Search  ^F Scope  ^U Clear  t Sessions  n/p Pages  N Rename  d Delete  r Refresh  i Metadata  pgup/pgdown Preview  esc Back"
		if m.options.Screen == "history" {
			text += "  q Quit"
		}
	case "settings":
		text = "esc Done  ↑↓←→ Move  enter Select  tab Next"
	case "stats":
		text = "esc Back  r Refresh"
	case "about":
		text = "esc Back  ↑↓ Scroll  pgup/pgdown Page  home/end Ends"
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
	case "help":
		text = "esc Back  / Search  ↑↓ Scroll  pgup/pgdown Page"
	case "log":
		text = "esc Back  pgup/pgdown Scroll"
	}
	if m.width < 76 && (m.screen == "vibe" || m.isActivity() || m.screen == "skills.sh") {
		text += "  v Details"
	}
	if m.focus == "filter" && m.screen != "about" && m.screen != "help" {
		text = "Type to search  enter Results  esc Done"
		if m.isActivity() {
			text = "^U Clear  ^F Scope  enter Results  esc Done  tab Done"
		}
	}
	if m.confirm != nil {
		text = "tab Choose  enter Accept  esc Cancel"
		if m.maxConfirmationScroll() > 0 {
			text = "↑↓ Read  tab Choose  enter Accept  esc Cancel"
		}
	}
	if m.rename != nil {
		return []string{"enter Save", "esc Cancel"}
	}
	if m.paletteOpen {
		return []string{"↑↓ Move", "enter Select", "esc Close"}
	}
	bindings := strings.Split(text, "  ")
	if m.confirm == nil {
		bindings = append(bindings, "^P Commands")
		if m.footerHelpBinding() == "? All keys" {
			bindings = append(bindings, "? All keys")
		}
	}
	return bindings
}

func (m *Model) footerHelpBinding() string {
	if m.focus != "" {
		return "^P Commands"
	}
	switch m.screen {
	case "compose", "create", "about", "settings", "sync":
		return "^P Commands"
	default:
		return "? All keys"
	}
}

func (m *Model) localizedBinding(binding string) string {
	key, label, found := strings.Cut(binding, " ")
	if !found {
		return key
	}
	return key + " " + m.tr(label)
}

func (m *Model) footerRows() []string {
	switch m.interfacePrefs.ShortcutMode() {
	case config.ShortcutBarOff:
		return nil
	case config.ShortcutBarFull:
		return m.fullFooterRows()
	}
	width := max(16, m.width-4)
	bindings := m.footerBindings()
	var primary []string
	help := m.footerHelpBinding()
	switch {
	case m.confirm != nil, m.rename != nil, m.paletteOpen:
		primary = bindings
		help = ""
	case m.screen == "help":
		primary = []string{"/ Search", "↑↓ Scroll"}
		help = "esc Back"
	case m.focus == "filter":
		primary = []string{"enter Results", "esc Done"}
		help = ""
	case m.screen == "activity":
		return []string{m.activityFooterRow(width, bindings[0], help)}
	case m.isActivity():
		primary = []string{bindings[0], "/ Search", "t Tabs"}
	case m.screen == "settings":
		primary = []string{"↑↓ Move", "enter Select", "esc Done"}
	default:
		primary = bindings[:min(3, len(bindings))]
	}
	if help != "" {
		primary = append(primary, help)
	}

	// Reserve space for help before adding whole action hints.
	row := ""
	if help != "" {
		row = m.localizedBinding(help)
		if ansi.StringWidth(row) > width {
			row, _, _ = strings.Cut(help, " ")
		}
		primary = primary[:len(primary)-1]
	}
	for i := len(primary) - 1; i >= 0; i-- {
		hint := m.localizedBinding(primary[i])
		if row == "" {
			row = hint
		} else if ansi.StringWidth(hint)+2+ansi.StringWidth(row) <= width {
			row = hint + "  " + row
		} else if help == "" {
			if m.confirm != nil && m.maxConfirmationScroll() > 0 {
				return []string{ansi.Truncate(m.localizedBinding("↑↓ Read"), width-9, "") + " tab↵ esc"}
			}
			if m.confirm != nil {
				return []string{"tab  ↵  esc"}
			}
			if m.paletteOpen {
				return []string{"↑↓  ↵  esc"}
			}
			return []string{"↵  esc"}
		}
	}
	return []string{ansi.Truncate(row, width, "")}
}

// Keep the session shortcuts and access to help on one row at every width.
func (m *Model) activityFooterRow(width int, open, help string) string {
	hints := []string{"N Rename", "K Stop", "X Stop all", help}
	parts := make([]string, len(hints))
	remaining := width - 2*(len(parts)-1)
	for i, hint := range hints {
		parts[i], _, _ = strings.Cut(hint, " ")
		remaining -= ansi.StringWidth(parts[i])
	}
	for i, hint := range hints {
		label := m.localizedBinding(hint)
		extra := ansi.StringWidth(label) - ansi.StringWidth(parts[i])
		if extra <= remaining {
			parts[i] = label
			remaining -= extra
		}
	}
	row := strings.Join(parts, "  ")
	for _, hint := range []string{"↑↓ Move", "/ Search", open} {
		label := m.localizedBinding(hint)
		if used := ansi.StringWidth(label) + 2; used <= remaining {
			row = label + "  " + row
			remaining -= used
		}
	}
	return row
}

func (m *Model) footerTop() int {
	height := max(8, m.height)
	switch m.interfacePrefs.ShortcutMode() {
	case config.ShortcutBarOff:
		return height
	case config.ShortcutBarFull:
		_, visible := m.fullFooterLayout()
		return height - visible
	default:
		return height - 1
	}
}

// Preserve each page's normal margins when Full mode uses additional rows.
func (m *Model) contentHeight() int {
	return min(max(8, m.height), m.footerTop()+1)
}

func (m *Model) footer(f *frame, a appearance) {
	footerStyle := a.panel
	shortcutStyle := a.panel.Foreground(lipgloss.Color(a.accent))
	if m.confirm != nil {
		footerStyle, _ = confirmationStyles(a)
		shortcutStyle = footerStyle.Bold(true)
	}
	rows := m.footerRows()
	if len(rows) == 0 {
		return
	}
	y := f.height - len(rows)
	f.fill(rect{0, y, f.width, len(rows)}, footerStyle)
	for index, row := range rows {
		f.put(2, y+index, footerStyle.Render(row))
		x := 2
		for _, binding := range strings.Split(row, "  ") {
			key := strings.Fields(binding)
			if len(key) > 0 {
				f.put(x, y+index, shortcutStyle.Render(key[0]))
			}
			x += ansi.StringWidth(binding) + 2
		}
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
	padding, y := 3, r.y+1
	if r.h == 1 {
		padding, y = 1, r.y
	} else {
		f.put(r.x, r.y+2, border.Render(strings.Repeat("─", r.w)))
	}
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
		shift = max(0, caret-max(1, r.w-padding-3))
	}
	f.put(r.x+padding, y, style.Render(ansi.Cut(value, shift, shift+max(1, r.w-padding-2))))
	if focused {
		glyph := " "
		if position < len(chars) {
			glyph = string(chars[position])
		}
		f.put(r.x+padding+caret-shift, y, a.cursor().Render(glyph))
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
	case "vibe", "activity", "sessions", "history", "skills.sh":
		m.browserView(f, a)
	case "sync":
		m.operationView(f, a)
	case "settings":
		m.settingsView(f, a)
	case "stats":
		m.statsView(f, a)
	case "about":
		m.aboutView(f, a)
	case "create":
		m.creatorView(f, a)
	case "help":
		m.helpView(f, a)
	case "community":
		m.communityView(f, a)
	case "create-preview", "log":
		r := m.readingRect()
		f.text(r, m.preview.View(), a.base, 0)

	}
	m.header(f, a)
	if m.status != "" { // Notifications occupy chrome, never replace a detail panel.
		text := ansi.Truncate(m.status, max(1, f.width-4), "…")
		f.put(max(1, f.width-ansi.StringWidth(text)-2), m.footerTop()-1, a.panel.Render(text))
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
	text := a.title.Render(m.tr("◆ Analytics")) + "\n\n  " + a.muted.Render(m.tr("Total invocations")) + "   " + a.base.Bold(true).Render(fmt.Sprint(m.stats.Total))
	heading := a.title.Foreground(lipgloss.Color(terminalColor(m.themePalette()["accent-lighten-1"])))
	accent := m.themePalette()["accent"]
	if len(m.stats.TopSkills) == 0 {
		text += "\n\n" + a.muted.Render(m.tr("No invocations recorded yet."))
	} else {
		text += "\n\n" + heading.Render(m.tr("Top skills"))
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
		text += "\n\n" + heading.Render(m.tr("By agent"))
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
