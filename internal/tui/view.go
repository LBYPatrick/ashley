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
	return appearance{base: base, border: base.Foreground(lipgloss.Color(border)), title: base.Foreground(lipgloss.Color(accent)).Bold(true), muted: base.Foreground(lipgloss.Color(muted)), selected: base.Background(lipgloss.Color(blendColor(accent, bg, .20))).Bold(true), blurred: base.Background(lipgloss.Color(blendColor(accent, bg, .3))), panel: base.Background(lipgloss.Color(panel)), accent: accent, bg: bg, fg: fg}
}

type screenLayout struct{ left, right, list, detail, input, mode rect }

func (m *Model) layout() screenLayout {
	w, h := max(20, m.width), max(8, m.height)
	margin := max(2, (w-132)/2)
	available := w - 2*margin
	sidebar := 28
	if m.screen == "sessions" {
		sidebar = 34
	}
	if m.screen == "history" {
		sidebar = 38
	}
	sidebar = min(sidebar, max(12, available*2/5))
	bottom := h - 2
	if m.screen == "vibe" {
		bottom = h - 7
	}
	if m.screen == "history" {
		bottom = h - 6
	}
	l := screenLayout{}
	l.left = rect{margin, 2, sidebar, max(3, bottom-2)}
	l.right = rect{margin + sidebar + 3, 2, max(1, available-sidebar-3), max(3, bottom-2)}
	l.list = rect{margin, 5, sidebar, max(1, bottom-5)}
	l.detail = rect{l.right.x + 1, 3, max(1, l.right.w-2), max(1, bottom-4)}
	l.input = rect{margin, h - 5, available, 3}
	l.mode = rect{margin, h - 6, available, 1}
	if m.maximized {
		l.list = rect{margin, 5, available, max(1, bottom-5)}
		l.left.w = available
		l.right = rect{}
		l.detail = rect{}
	}
	return l
}
func (m *Model) header(f *frame, a appearance) {
	f.fill(rect{0, 0, f.width, 1}, a.panel)
	title := "Ashley v" + ashley.Version() + "  /  " + screenName(m.screen)
	f.put(2, 0, a.panel.Bold(true).Render(ansi.Truncate(title, max(1, f.width-14), "…")))
	f.put(f.width-10, 0, a.panel.Render(time.Now().Format("15:04:05")))
}
func (m *Model) footer(f *frame, a appearance) {
	text := "enter Open  ↑↓ Move  q Quit"
	switch m.screen {
	case "vibe":
		text = "esc Back  / Filter  m Mode  p Copy Prompt"
	case "sessions":
		text = "esc Back  c Copy ID  l Log  s Sort  K Kill  X Kill All  d Delete  r Refresh  k Cleanup"
		if m.options.Screen == "sessions" {
			text = strings.Replace(text, "esc Back", "q Quit", 1)
		}
	case "history":
		text = "esc Back  n Next  p Prev  / Search  d Delete  r Refresh"
		if m.options.Screen == "history" {
			text = "q Quit  n Next  p Prev  / Search  d Delete  r Refresh"
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
	case "generate":
		text = "esc Back  r Generate again  ↑↓ Scroll"
	case "install":
		text = "esc Back  enter Install detected  i Set up CLI"
	case "help", "log":
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
	f.fill(r, a.panel)
	f.put(r.x, r.y+2, border.Render(strings.Repeat("─", r.w)))
	style := a.panel
	if value == "" {
		value = placeholder
		style = a.muted.Background(a.panel.GetBackground())
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

// View composes every page with shared chrome and bounded terminal cells.
func (m *Model) View() string {
	a := m.appearance()
	f := newFrame(max(20, m.width), max(8, m.height), a.base)
	m.header(f, a)
	switch m.screen {
	case "hub", "vibe", "sessions", "history":
		m.browserView(f, a)
	case "generate", "install":
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
	if m.status != "" { // Notifications occupy chrome, never replace a detail panel.
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
	if !m.maximized {
		for y := 3; y < f.height-3; y++ {
			f.put(l.right.x-2, y, a.border.Render("│"))
		}
	}
	heading := "Home"
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
		heading = fmt.Sprintf("Sessions (%d)", len(labels))
		f.put(l.left.x, 3, a.muted.Render(ansi.Truncate(fmt.Sprintf("%d running · %s", running, sort), l.left.w, "…")))
	case "history":
		heading = fmt.Sprintf("History (%d) — Page %d/%d", m.total, m.offset/50+1, max(1, (m.total+49)/50))
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
	hx, hy := l.left.x, 2
	if m.screen == "history" {
		heading = fmt.Sprintf("History (%d)", m.total)
		f.put(hx, 3, a.muted.Render(ansi.Truncate(fmt.Sprintf("Page %d/%d · n next / p prev", m.offset/50+1, max(1, (m.total+49)/50)), l.left.w, "…")))
	}
	f.put(hx, hy, a.base.Bold(true).Render(ansi.Truncate(heading, max(1, l.left.w), "…")))
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
		marker := "  "
		if index == m.cursor {
			marker = "› "
		}
		label := marker + labels[index]
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
		f.put(log.x, log.y, a.border.Render(strings.Repeat("─", log.w)))
		f.put(log.x, log.y+1, a.title.Render(ansi.Truncate("Log (last 50 lines)", max(1, log.w), "")))
		f.text(body, m.sessionLog, a.base, min(m.logOffset, m.maxLogOffset()))
	} else {
		f.richText(l.detail, detail, a, m.preview.YOffset)
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
		if l.mode.w < 70 {
			f.put(l.mode.x, l.mode.y, a.muted.Render(ansi.Truncate("Mode: "+[]string{"Normal", "DSP", "AUTO", "AFK"}[m.mode]+" · m change · "+agents.Get(m.agent).Label, l.mode.w, "…")))
		} else {
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
		}
		f.input(l.input, m.question.Value(), "Enter your question, then press Enter to run...", m.focus == "question", a, m.question.Position())
		if m.focus == "filter" {
			f.input(rect{l.left.x, l.left.y, l.left.w, 3}, m.filter.Value(), "Filter skills...", true, a)
		}
	} else if m.screen == "history" {
		f.input(l.input, m.filter.Value(), "Search history (skill, question, directory)...", m.focus == "filter", a, m.filter.Position())
	}
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
	return text
}
