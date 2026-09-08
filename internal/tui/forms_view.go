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

// Settings use a readable column even in an ultrawide terminal.
func (m *Model) settingsBounds() rect {
	width := min(86, max(20, m.width)-4)
	return rect{(max(20, m.width) - width) / 2, 2, width, 28}
}
func (m *Model) settingsControls() []settingControl {
	b := m.settingsBounds()
	x, inner := b.x+2, b.w-4
	var controls []settingControl
	for index, key := range agents.Keys() {
		controls = append(controls, settingControl{rect{x, 9 + index, inner, 1}, 0, index, key, agents.Get(key).Label})
	}
	modeWidth := min(12, (inner-2)/2)
	controls = append(controls, settingControl{rect{x, 18, modeWidth, 1}, 1, 0, "dark", "Dark"}, settingControl{rect{x + modeWidth + 2, 18, modeWidth, 1}, 1, 1, "light", "Light"})
	// Narrow terminals use fewer columns; semantic keys stay stable on resize.
	columns := min(5, max(1, (inner+2)/15))
	for index, p := range config.Presets() {
		col := index % columns
		width := (inner - (columns-1)*2) / columns
		controls = append(controls, settingControl{rect{x + col*(width+2), 23 + index/columns*2, width, 1}, 2 + index/5, index % 5, p.Key, strings.Title(p.Key)})
	}
	bottom := 23 + (9/columns)*2
	controls = append(controls, settingControl{rect{x + max(0, inner-12), bottom + 3, min(12, inner), 1}, 4, 0, "done", "Done"})
	return controls
}
func (m *Model) settingsContentHeight() int {
	controls := m.settingsControls()
	return controls[len(controls)-1].y + 3
}
func (m *Model) settingsView(f *frame, a appearance) {
	content := newFrame(f.width, m.settingsContentHeight(), a.base)
	b := m.settingsBounds()
	x, inner := b.x+2, b.w-4
	title := "Settings"
	subtitle := "Make Ashley feel like yours. Changes save automatically."
	if m.firstRun {
		title = "Welcome to Ashley"
		subtitle = "Choose your coding agent and appearance to get started."
	}
	content.put(x, 2, a.base.Bold(true).Render(title))
	content.text(rect{x, 4, inner, 2}, subtitle, a.muted, 0)
	for _, section := range []struct {
		y     int
		label string
	}{{7, "Coding agent"}, {16, "Appearance"}, {21, "Accent color"}} {
		content.put(x, section.y, a.base.Bold(true).Render(section.label))
	}
	surface := a.base.Background(lipgloss.Color(terminalColor(themeValues[m.theme.Mode+"-"+m.theme.Preset]["surface-lighten-1"])))
	for _, control := range m.settingsControls() {
		r := control.rect
		selected := control.key == m.agent || control.key == m.theme.Mode || control.key == m.theme.Preset
		focused := control.row == m.settingsRow && control.column == m.settingsColumn
		style := surface
		if focused {
			style = style.Background(lipgloss.Color(blendColor(a.accent, a.bg, .18)))
		}
		if selected {
			style = style.Foreground(lipgloss.Color(a.accent)).Bold(true)
			if m.theme.Mode == "light" {
				style = style.Foreground(lipgloss.Color(a.fg))
			}
		}
		if control.row == 4 {
			style = a.selected.Foreground(lipgloss.Color("#161616"))
		}
		content.fill(r, style)
		marker := " "
		if focused {
			marker = "›"
		}
		content.put(r.x, r.y, style.Render(marker))
		left := r.x + 2
		if control.row == 2 || control.row == 3 {
			preset := config.Presets()[(control.row-2)*5+control.column]
			content.put(left, r.y, style.Foreground(lipgloss.Color(terminalColor(preset.Primary))).Render("█"))
			content.put(left+1, r.y, style.Foreground(lipgloss.Color(terminalColor(preset.Accent))).Render("█"))
			left += 3
		}
		label := ansi.Truncate(control.label, max(1, r.x+r.w-left-2), "…")
		content.put(left, r.y, style.Render(label))
		if selected {
			content.put(r.x+r.w-2, r.y, style.Render("✓"))
		}
	}
	for y := 1; y < f.height-1; y++ {
		source := y + m.screenScroll
		if source < content.height {
			f.put(0, y, content.row(source))
		}
	}
	if content.height > f.height {
		f.scrollbar(rect{b.x + b.w, 1, 1, f.height - 2}, content.height-2, m.screenScroll, a)
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
	m.screenScroll = max(0, min(max(0, m.settingsContentHeight()-m.height), m.screenScroll))
}

// Directional focus follows the positions on screen, including wrapped grids.
func (m *Model) moveSettingFocus(key string) {
	controls := m.settingsControls()
	current := 0
	for i, c := range controls {
		if c.row == m.settingsRow && c.column == m.settingsColumn {
			current = i
			break
		}
	}
	target := current
	if key == "tab" {
		target = (current + 1) % len(controls)
	} else if key == "shift+tab" {
		target = (current + len(controls) - 1) % len(controls)
	} else {
		from := controls[current]
		best := int(^uint(0) >> 1)
		for i, c := range controls {
			dx, dy := c.x-from.x, c.y-from.y
			score := best
			switch key {
			case "up":
				if dy < 0 {
					score = -dy*1000 + abs(dx)
				}
			case "down":
				if dy > 0 {
					score = dy*1000 + abs(dx)
				}
			case "left":
				if dy == 0 && dx < 0 {
					score = -dx
				}
			case "right":
				if dy == 0 && dx > 0 {
					score = dx
				}
			}
			if score < best {
				best = score
				target = i
			}
		}
	}
	m.settingsRow, m.settingsColumn = controls[target].row, controls[target].column
}
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
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
	m.settingsRow, m.settingsColumn = 0, 0
	for i, key := range agents.Keys() {
		if key == m.agent {
			m.settingsColumn = i
			break
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
