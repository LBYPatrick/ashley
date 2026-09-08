package tui

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	ashley "github.com/LBYPatrick/ashley"
	"github.com/LBYPatrick/ashley/internal/agents"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type appearance struct {
	base, border, title, muted, selected, blurred, panel lipgloss.Style
	accent, bg, fg                                       string
}

func (m *Model) appearance() appearance {
	values := themeValues[m.theme.Mode+"-"+m.theme.Preset]
	if values == nil {
		values = themeValues["dark-blue"]
	}
	accent := terminalColor(values["accent"])
	bg, border, panel := values["surface"], values["surface-lighten-2"], terminalColor(values["panel"])
	contrast := "#FFFFFF"
	if m.theme.Mode == "light" {
		contrast = "#000000"
	}
	fg, muted := blendColor(contrast, bg, .87), blendColor(contrast, bg, .60)
	base := lipgloss.NewStyle().Background(lipgloss.Color(bg)).Foreground(lipgloss.Color(fg))
	return appearance{base: base, border: base.Foreground(lipgloss.Color(border)), title: base.Foreground(lipgloss.Color(accent)).Bold(true), muted: base.Foreground(lipgloss.Color(muted)), selected: base.Background(lipgloss.Color(accent)).Foreground(lipgloss.Color(bg)).Bold(true), blurred: base.Background(lipgloss.Color(blendColor(accent, bg, .3))), panel: base.Background(lipgloss.Color(panel)), accent: accent, bg: bg, fg: fg}
}

type screenLayout struct{ left, right, list, detail, input, mode rect }

func (m *Model) layout() screenLayout {
	w, h := max(20, m.width), max(8, m.height)
	left := 38
	switch m.screen {
	case "vibe":
		left = 30
	case "sessions":
		left = 42
	case "history":
		left = 52
	}
	// Keep both panels operable on narrow terminals while retaining the original
	// fixed sidebar widths at normal terminal sizes.
	left = min(left, max(16, w/2-2))
	if w >= 90 {
		switch m.screen {
		case "hub":
			left = 38
		case "vibe":
			left = 30
		case "sessions":
			left = 42
		case "history":
			left = 52
		}
	}
	bottom := h - 2
	if m.screen == "vibe" {
		bottom = h - 7
	}
	if m.screen == "history" {
		bottom = h - 6
	}
	l := screenLayout{left: rect{1, 2, left, max(3, bottom-2)}, right: rect{left + 2, 2, max(3, w-left-3), max(3, bottom-2)}}
	if m.screen == "vibe" {
		l.right.w -= 2
	}
	l.list = rect{3, 5, left - 4, max(1, l.left.h-4)}
	padding, top := 3, 2
	if m.screen == "hub" {
		top = 3
	}
	if m.screen == "sessions" || m.screen == "history" {
		padding = 2
	}
	l.detail = rect{l.right.x + padding + 1, l.right.y + top, max(1, l.right.w-2*padding-2), max(1, l.right.h-top-2)}
	if m.screen == "vibe" {
		l.detail.w -= 2
	}
	l.input = rect{2, h - 5, w - 4, 3}
	l.mode = rect{2, h - 6, w - 4, 1}
	if m.screen == "history" && m.options.Screen != "history" {
		left = min(50, max(16, w/2))
		l.left = rect{0, 1, left, h - 5}
		l.right = rect{left, 1, w - left, h - 5}
		l.list = rect{1, 3, left - 3, h - 7}
		l.detail = rect{left + 2, 2, w - left - 4, h - 6}
		l.input = rect{1, h - 3, w - 2, 3}
	}
	if m.maximized {
		l.left = rect{1, 2, w - 2, h - 4}
		l.list = rect{3, 5, w - 6, h - 8}
		l.right = rect{}
		l.detail = rect{}
	}
	return l
}
func (m *Model) header(f *frame, a appearance) {
	title := "Ashley v" + ashley.Version()
	subtitle := "Interactive Skill Set for Coding Agents"
	if m.options.Screen == "sessions" {
		title = "Ashley Sessions"
		subtitle = "Manage detached coding-agent sessions"
	}
	if m.options.Screen == "history" {
		title = "Ashley History"
		subtitle = "Invocation log"
	}
	if m.options.Screen == "create" {
		subtitle = "Create New Skill"
	}
	f.fill(rect{0, 0, f.width, 1}, a.panel)
	f.put(1, 0, a.panel.Foreground(lipgloss.Color(a.accent)).Bold(true).Render("⭘"))
	text := title + " — " + subtitle
	available := max(1, f.width-20)
	text = ansi.Truncate(text, available, "…")
	f.put(max(4, (f.width-ansi.StringWidth(text))/2-1), 0, a.panel.Foreground(lipgloss.Color(a.accent)).Bold(true).Render(text))
	if m.screen != "history" && m.screen != "create" {
		f.put(f.width-9, 0, a.panel.Render(time.Now().Format("15:04:05")))
	}
}
func (m *Model) footer(f *frame, a appearance) {
	text := "q Quit"
	switch m.screen {
	case "vibe":
		text = "esc Back  / Filter  m Mode  p Copy Prompt"
	case "sessions":
		text = "esc Back  c Copy ID  l Log  s Sort  K Kill  X Kill All  d Delete  r Refresh  k Cleanup"
		if m.options.Screen == "sessions" {
			text = strings.Replace(text, "esc Back", "q Quit", 1)
		}
	case "history":
		text = "esc Back  / Search  d Delete  r Refresh  n Next"
		if m.options.Screen == "history" {
			text = "q Quit  / Search  r Refresh  d Delete  n Next  p Prev"
		}
	case "settings":
		text = "esc Done  ↑↓←→ Move  enter Select  tab Next"
	case "stats":
		text = "esc Back  r Refresh"
	case "create", "create-preview":
		text = "esc Back/Cancel  ^s Save"
	case "log":
		text = "esc Back  pgup/pgdown Scroll"
	}
	y := f.height - 1
	f.fill(rect{0, y, f.width, 1}, a.panel)
	f.put(1, y, a.panel.Render(ansi.Truncate(text, max(1, f.width-13), "…")))
	f.put(max(0, f.width-12), y, a.panel.Render("▏^p palette"))
	x := 1
	for _, binding := range strings.Split(text, "  ") {
		key := strings.Fields(binding)
		if len(key) > 0 && x+len(key[0]) < f.width-12 {
			f.put(x, y, a.panel.Foreground(lipgloss.Color(a.accent)).Render(key[0]))
		}
		x += ansi.StringWidth(binding) + 2
	}
	f.put(max(0, f.width-11), y, a.panel.Foreground(lipgloss.Color(a.accent)).Render("^p"))
}
func (f *frame) input(r rect, value, placeholder string, focused bool, a appearance, cursor ...int) {
	if r.w < 4 {
		return
	}
	border := a.border
	if focused {
		border = a.title
	}
	f.put(r.x, r.y, border.Render("▊"+strings.Repeat("▔", r.w-2)+"▎"))
	f.put(r.x, r.y+2, border.Render("▊"+strings.Repeat("▁", r.w-2)+"▎"))
	f.put(r.x, r.y+1, border.Render("▊"))
	f.put(r.x+r.w-1, r.y+1, border.Render("▎"))
	style := a.base
	if value == "" {
		value = placeholder
		style = a.muted
	}
	f.put(r.x+3, r.y+1, style.Render(ansi.Truncate(value, max(1, r.w-5), "")))
	if focused {
		position := len([]rune(value))
		if len(cursor) > 0 {
			position = cursor[0]
		}
		position = min(max(0, position), max(0, r.w-6))
		glyph := " "
		chars := []rune(value)
		if position < len(chars) {
			glyph = string(chars[position])
		}
		f.put(r.x+3+position, r.y+1, a.base.Reverse(true).Render(glyph))
	}
}

// View preserves the original Textual panel geometry and information hierarchy.
func (m *Model) View() string {
	a := m.appearance()
	f := newFrame(max(20, m.width), max(8, m.height), a.base)
	m.header(f, a)
	switch m.screen {
	case "hub", "vibe", "sessions", "history":
		m.browserView(f, a)
	case "settings":
		m.settingsView(f, a)
	case "stats":
		m.statsView(f, a)
	case "create":
		m.creatorView(f, a)
	case "create-preview", "log":
		r := rect{1, 2, f.width - 2, f.height - 4}
		f.box(r, a.border)
		f.text(rect{4, 4, r.w - 6, r.h - 3}, m.preview.View(), a.base, 0)
	}
	if m.status != "" && m.status != "Completed." { // Notifications occupy chrome, never replace a detail panel.
		text := ansi.Truncate(m.status, max(1, f.width-4), "…")
		f.put(max(1, f.width-ansi.StringWidth(text)-2), f.height-2, a.panel.Render(text))
	}
	m.footer(f, a)
	if m.paletteOpen {
		m.paletteView(f, a)
	}
	return f.String()
}
func (m *Model) browserView(f *frame, a appearance) {
	l := m.layout()
	embeddedHistory := m.screen == "history" && m.options.Screen != "history"
	if embeddedHistory {
		for y := 1; y < f.height-4; y++ {
			f.put(l.left.w-1, y, a.border.Render("│"))
		}
		f.put(0, f.height-4, a.border.Render(strings.Repeat("─", f.width)))
	} else {
		f.box(l.left, a.border)
		f.box(l.right, a.border)
	}
	heading := "Ashley"
	labels := featureTitles
	switch m.screen {
	case "vibe":
		heading = "Skills"
		labels = nil
		for _, name := range m.names {
			labels = append(labels, "a-"+name)
		}
	case "sessions":
		running := 0
		labels = nil
		for _, s := range m.sessionRows {
			icon := "○"
			if m.sessionAlive[s.ID] {
				icon = "●"
				running++
			}
			labels = append(labels, fmt.Sprintf("%s %s  %s  (%s)", icon, s.ID, s.Skill, s.Elapsed(time.Now())))
		}
		sort := "newest"
		if m.sortMode == "skill" {
			sort = "skill"
		}
		heading = fmt.Sprintf("Sessions (%d) · %d running · sort: %s", len(labels), running, sort)
	case "history":
		heading = fmt.Sprintf("History (%d) — Page %d/%d", m.total, m.offset/50+1, max(1, (m.total+49)/50))
		if !embeddedHistory {
			heading = fmt.Sprintf("History (%d total) — Page %d/%d", m.total, m.offset/50+1, max(1, (m.total+49)/50))
		}
		labels = nil
		for _, v := range m.historyRows {
			stamp := v.TimeDisplay()
			if len(stamp) > 16 {
				stamp = stamp[5:16]
			}
			arrow := ""
			if v.Detached {
				arrow = " ⇢"
			}
			labels = append(labels, stamp+"  "+v.Skill+arrow+"  "+ansi.Truncate(v.QuestionShort(), 30, ""))
		}
	}
	hx, hy := l.left.x+2, l.left.y+1
	if embeddedHistory {
		hx, hy = 1, 2
	}
	f.put(hx, hy, a.title.Render(ansi.Truncate(heading, max(1, l.left.w-4), "")))
	start := max(0, m.cursor-l.list.h+1)
	for index := start; index < min(len(labels), start+l.list.h); index++ {
		style := a.base
		if index == m.cursor {
			style = a.selected
			if m.focus != "" {
				style = a.blurred
			}
		}
		// ListItem and its Label each contribute one horizontal padding cell.
		label := "  " + labels[index]
		label = ansi.Truncate(label, l.list.w, "")
		label += strings.Repeat(" ", max(0, l.list.w-ansi.StringWidth(label)))
		f.put(l.list.x, l.list.y+index-start, style.Render(label))
	}
	if m.maximized {
		return
	}
	detail := m.detailText()
	if m.screen == "sessions" {
		detailRect, log, body := m.sessionPanels()
		f.text(detailRect, detail, a.base, m.preview.YOffset)
		f.box(log, a.border)
		f.put(log.x+3, log.y+1, a.title.Render(ansi.Truncate("Log (last 50 lines)", max(1, log.w-5), "")))
		f.text(body, m.sessionLog, a.base, min(m.logOffset, m.maxLogOffset()))
	} else {
		f.text(l.detail, detail, a.base, m.preview.YOffset)
		// Headings use the same accent as the original rich-text panels.
		lines := strings.Split(ansi.Wrap(detail, l.detail.w, ""), "\n")
		for index := m.preview.YOffset; index < min(len(lines), m.preview.YOffset+l.detail.h); index++ {
			if m.screen == "vibe" && strings.HasPrefix(lines[index], "   ") && len(strings.Fields(lines[index])) > 1 {
				number := strings.Fields(lines[index])[0]
				if _, err := strconv.Atoi(number); err == nil {
					f.put(l.detail.x+2, l.detail.y+index-m.preview.YOffset, a.title.Render(fmt.Sprintf("%2s", number)))
				}
			}
			if index == 0 || lines[index] == "Overview" || lines[index] == "Workflow" {
				f.put(l.detail.x, l.detail.y+index-m.preview.YOffset, a.title.Render(lines[index]))
			}
		}
	}
	if m.screen == "vibe" {
		virtual := len(strings.Split(ansi.Wrap(detail, l.detail.w, ""), "\n"))
		if virtual > l.detail.h {
			f.scrollbar(rect{l.detail.x + l.detail.w, l.detail.y, 2, l.detail.h}, virtual, m.preview.YOffset, a)
		}
		x := l.mode.x
		f.put(x, l.mode.y, a.muted.Render("Run mode "))
		x += 9
		for index, label := range []string{"Normal", "DSP", "AUTO", "AFK"} {
			icon := "○"
			style := a.base
			if index == m.mode {
				icon = "●"
				style = a.selected
			}
			text := " " + icon + " " + label + " "
			f.put(x, l.mode.y, style.Render(text))
			x += ansi.StringWidth(text) + 1
		}
		hint := []string{"Standard permission prompts", "Skip all permission checks", "Auto-accept edits", "Fully autonomous — implies DSP"}[m.mode] + " · " + agents.Get(m.agent).Label
		available := max(0, f.width-x-3)
		hint = ansi.Truncate(hint, available, "…")
		f.put(f.width-ansi.StringWidth(hint)-2, l.mode.y, a.muted.Render(hint))
		f.input(l.input, m.question.Value(), "Enter your question, then press Enter to run...", m.focus == "question", a, m.question.Position())
		if m.focus == "filter" {
			f.input(rect{l.left.x, l.left.y, l.left.w, 3}, m.filter.Value(), "Filter skills...", true, a)
		}
	} else if m.screen == "history" {
		f.input(l.input, m.filter.Value(), "Search history (skill, question, directory)...", m.focus == "filter", a, m.filter.Position())
	}
}
func (m *Model) statsView(f *frame, a appearance) {
	r := rect{1, 2, f.width - 2, f.height - 4}
	f.box(r, a.border)
	text := a.title.Render("◆ Analytics") + "\n\n  " + a.muted.Render("Total invocations") + "   " + a.base.Bold(true).Render(fmt.Sprint(m.stats.Total))
	heading := a.title.Foreground(lipgloss.Color(terminalColor(themeValues[m.theme.Mode+"-"+m.theme.Preset]["accent-lighten-1"])))
	accent := themeValues[m.theme.Mode+"-"+m.theme.Preset]["accent"]
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
	f.text(rect{5, 4, max(1, r.w-8), r.h - 3}, text, a.base, m.screenScroll)
}
