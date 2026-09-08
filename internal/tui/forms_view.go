package tui

import (
	"strings"

	"github.com/LBYPatrick/ashley/internal/agents"
	"github.com/LBYPatrick/ashley/internal/config"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type settingControl struct {
	rect
	row, column int
	key, label  string
}

func (m *Model) settingsControls() []settingControl {
	width := max(20, m.width) - 2
	if m.height < 38 {
		width -= 2
	}
	inner := max(9, width-8)
	var result []settingControl
	for index, key := range agents.Keys() {
		col := index % 3
		x := 5 + col*inner/3
		y := 10 + index/3*3
		w := (col+1)*inner/3 - col*inner/3 - 1
		result = append(result, settingControl{rect{x, y, max(3, w), 3}, 0, index, key, agents.Get(key).Label})
	}
	result = append(result, settingControl{rect{5, 19, 12, 3}, 1, 0, "dark", "Dark"}, settingControl{rect{19, 19, 13, 3}, 1, 1, "light", "Light"})
	for index, p := range config.Presets() {
		col := index % 5
		x := 5 + col*(inner-4)/5 + col
		w := (col+1)*(inner-4)/5 - col*(inner-4)/5
		result = append(result, settingControl{rect{x, 24 + index/5*4, max(3, w), 3}, 2 + index/5, col, p.Key, strings.Title(p.Key)})
	}
	label := "Save & Close"
	if m.firstRun {
		label = "Done"
	}
	result = append(result, settingControl{rect{5, 31, len(label) + 10, 3}, 4, 0, "done", label})
	return result
}
func (m *Model) settingsView(f *frame, a appearance) {
	// Draw the full scrollable card first; then clip it into the content area.
	content := newFrame(f.width, 38, a.base)
	cardwidth := f.width - 2
	if m.height < 38 {
		cardwidth -= 2
	}
	content.box(rect{1, 2, cardwidth, 34}, a.border)
	intro := "Adjust Ashley's preferences. Changes apply instantly."
	if m.firstRun {
		intro = "Welcome to Ashley — pick your agent and a look to get started."
	}
	content.text(rect{5, 4, max(1, cardwidth-8), 3}, intro+"\nArrows move · Enter selects · Esc saves & exits", a.muted, 0)
	content.put(5, 8, a.title.Render("Coding agent"))
	content.put(5, 17, a.title.Render("Mode"))
	content.put(5, 22, a.title.Render("Primary colour / preset"))
	for _, control := range m.settingsControls() {
		r := control.rect
		style := a.base.Background(lipgloss.Color(themeValues[m.theme.Mode+"-"+m.theme.Preset]["surface-lighten-1"]))
		label := control.label
		selected := control.key == m.agent || control.key == m.theme.Mode || control.key == m.theme.Preset
		if control.row < 2 {
			mark := "○ "
			if selected {
				mark = "● "
				style = a.selected
			}
			label = mark + label
		}
		if control.row == 2 || control.row == 3 {
			for _, p := range config.Presets() {
				if p.Key == control.key {
					style = style.Background(lipgloss.Color(terminalColor(p.Primary))).Foreground(lipgloss.Color("#FFFFFF"))
					if p.Primary != p.Accent {
						content.fill(rect{r.x, r.y, 1, r.h}, a.base.Background(lipgloss.Color(terminalColor(p.Accent))))
					}
				}
			}
			if selected {
				label = "✓ " + label
			}
		}
		if control.row == 4 {
			style = a.selected
		}
		content.fill(r, style)
		if control.row == 2 || control.row == 3 {
			for _, p := range config.Presets() {
				if p.Key == control.key && p.Primary != p.Accent {
					content.fill(rect{r.x, r.y, 1, r.h}, a.base.Background(lipgloss.Color(terminalColor(p.Primary))).Foreground(lipgloss.Color(terminalColor(p.Accent))))
					for y := r.y; y < r.y+r.h; y++ {
						content.put(r.x, y, a.base.Background(lipgloss.Color(terminalColor(p.Primary))).Foreground(lipgloss.Color(terminalColor(p.Accent))).Render("█"))
					}
				}
			}
		}
		label = ansi.Truncate(label, max(1, r.w-2), "…")
		content.put(r.x+max(0, (r.w-ansi.StringWidth(label))/2), r.y+1, style.Bold(true).Render(label))
		if selected && control.row >= 2 && control.row <= 3 {
			outline := style.Foreground(lipgloss.Color(a.fg))
			content.put(r.x, r.y, outline.Render("█"+strings.Repeat("▀", max(0, r.w-2))+"█"))
			content.put(r.x, r.y+2, outline.Render("█"+strings.Repeat("▄", max(0, r.w-2))+"█"))
			content.put(r.x, r.y+1, outline.Render("█"))
			content.put(r.x+r.w-1, r.y+1, outline.Render("█"))
		} else if control.row == m.settingsRow && control.column == m.settingsColumn {
			content.box(r, style.Foreground(lipgloss.Color(a.fg)))
		}
	}
	for y := 1; y < f.height-1; y++ {
		source := y + m.screenScroll
		if source < content.height {
			f.put(0, y, content.row(source))
		}
	}
	if m.height < 38 {
		f.scrollbar(rect{f.width - 2, 1, 2, f.height - 2}, 36, m.screenScroll, a)
	}
}
func (m *Model) keepSettingVisible() {
	for _, control := range m.settingsControls() {
		if control.row == m.settingsRow && control.column == m.settingsColumn {
			if control.y-m.screenScroll < 2 {
				m.screenScroll = max(0, control.y-2)
			}
			if control.y+control.h-m.screenScroll > m.height-2 {
				m.screenScroll = control.y + control.h - m.height + 2
			}
		}
	}
}
func (m *Model) creatorView(f *frame, a appearance) {
	if m.wizard == nil {
		f.text(rect{2, 2, f.width - 4, f.height - 4}, m.editor.View(), a.base, 0)
		return
	}
	w := m.wizard
	if w.stage != 0 {
		f.text(rect{2, 2, f.width - 4, f.height - 4}, m.wizardView(), a.base, m.screenScroll)
		return
	}
	content := newFrame(f.width, 45, a.base)
	content.put(2, 3, a.title.Render("Step 1/4 — Basics"))
	content.put(2, 6, a.base.Bold(true).Render("Skill Name"))
	content.put(2, 7, a.muted.Render("Lowercase, no spaces (e.g., 'lint-fix')"))
	content.put(2, 13, a.base.Bold(true).Render("Description"))
	content.put(2, 18, a.base.Bold(true).Render("Extends (optional)"))
	names, _ := m.options.Catalog.Names()
	content.text(rect{2, 19, f.width - 4, 2}, "Available: "+strings.Join(names, ", "), a.muted, 0)
	placeholders := []string{"my-skill", "What this skill does (shown in skill list)", "(leave empty for standalone)", ""}
	for index, y := range []int{9, 14, 22, 29} {
		value := w.basics[index]
		if index == w.field {
			value = w.input.Value()
		}
		content.input(rect{2, y, f.width - 4, 3}, value, placeholders[index], index == w.field, a)
	}
	content.put(2, 26, a.base.Bold(true).Render("Preamble"))
	content.put(2, 27, a.muted.Render("System prompt intro for the agent"))
	content.put(2, 34, a.selected.Render("  Next →  "))
	content.put(15, 34, a.selected.Render("  Save  "))
	for y := 1; y < f.height-1; y++ {
		index := y + m.screenScroll
		if index < content.height {
			f.put(0, y, content.row(index))
		}
	}
}

func (m *Model) focusSettings() {
	for row, keys := range settingsRows() {
		if row == 2 || row == 3 {
			for column, key := range keys {
				if key == m.theme.Preset {
					m.settingsRow, m.settingsColumn = row, column
					return
				}
			}
		}
	}
}
func (m *Model) keepCreatorVisible() {
	if m.wizard == nil {
		return
	}
	if m.wizard.stage != 0 {
		m.screenScroll = 0
		return
	}
	y := []int{9, 14, 22, 29}[m.wizard.field]
	if y-m.screenScroll < 2 {
		m.screenScroll = max(0, y-2)
	}
	if y+3-m.screenScroll > m.height-2 {
		m.screenScroll = max(0, y+3-m.height+2)
	}
}
