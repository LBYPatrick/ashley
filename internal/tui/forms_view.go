package tui

import (
	"slices"
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
	return rect{2, 2, width, 28}
}

const (
	settingAgent = iota
	settingAppearance
	settingAccent
	settingShortcutBar
	settingLanguage
	settingDone
)

type settingsSection struct {
	rect
	label string
}

func (m *Model) settingsLayout() ([]settingsSection, []settingControl) {
	b := m.settingsBounds()
	x, inner := b.x, b.w
	left := inner
	if inner >= 70 {
		left = inner * 3 / 5
	}
	sections := []settingsSection{
		{rect{x, 7, left, 1}, "Coding agent"},
		{rect{x, 16, left, 1}, "Appearance"},
	}
	var controls []settingControl
	for index, key := range agents.Keys() {
		controls = append(controls, settingControl{rect{x, 9 + index, left, 1}, settingAgent, index, key, agents.Get(key).Label})
	}
	modeColumns := min(3, max(1, (left+2)/12))
	modeWidth := min(12, (left-(modeColumns-1)*2)/modeColumns)
	for i, mode := range []string{"auto", "dark", "light"} {
		controls = append(controls, settingControl{rect{x + i%modeColumns*(modeWidth+2), 18 + i/modeColumns*2, modeWidth, 1}, settingAppearance, i, mode, m.tr(strings.Title(mode))})
	}
	accentY := 21 + (2/modeColumns)*2
	sections = append(sections, settingsSection{rect{x, accentY, left, 1}, "Accent color"})
	columns := min(5, max(1, (left+2)/15))
	for index, p := range config.Presets() {
		col := index % columns
		width := (left - (columns-1)*2) / columns
		controls = append(controls, settingControl{rect{x + col*(width+2), accentY + 2 + index/columns, width, 1}, settingAccent, index, p.Key, m.tr(strings.Title(p.Key))})
	}
	bottom := accentY + 2 + (len(config.Presets())-1)/columns

	prefsX, prefsY, prefsWidth := x, bottom+2, inner
	if left < inner {
		prefsX, prefsY, prefsWidth = x+left+4, 7, inner-left-4
	}
	sections = append(sections, settingsSection{rect{prefsX, prefsY, prefsWidth, 1}, "Language"})
	languageColumns := min(3, max(1, prefsWidth/16))
	languageWidth := (prefsWidth - (languageColumns-1)*2) / languageColumns
	for i, language := range languages {
		label := languageNames[i]
		if language == "auto" {
			label = m.tr(label)
		}
		r := rect{prefsX + i%languageColumns*(languageWidth+2), prefsY + 2 + i/languageColumns, languageWidth, 1}
		controls = append(controls, settingControl{r, settingLanguage, i, language, label})
	}
	shortcutY := prefsY + 4 + (len(languages)-1)/languageColumns
	sections = append(sections, settingsSection{rect{prefsX, shortcutY, prefsWidth, 1}, "Shortcut bar"})
	shortcutColumns := 1
	if prefsWidth >= 46 {
		shortcutColumns = 3
	}
	shortcutWidth := (prefsWidth - (shortcutColumns-1)*2) / shortcutColumns
	for i, mode := range []config.ShortcutBarMode{config.ShortcutBarStandard, config.ShortcutBarFull, config.ShortcutBarOff} {
		r := rect{prefsX + i%shortcutColumns*(shortcutWidth+2), shortcutY + 2 + i/shortcutColumns, shortcutWidth, 1}
		controls = append(controls, settingControl{r, settingShortcutBar, i, string(mode), m.tr(strings.Title(string(mode)))})
	}
	bottom = max(bottom, shortcutY+2+2/shortcutColumns)
	controls = append(controls, settingControl{rect{x + inner - 12, bottom + 2, 12, 1}, settingDone, 0, "done", m.tr("Done")})
	return sections, controls
}

func (m *Model) settingsControls() []settingControl {
	_, controls := m.settingsLayout()
	return controls
}

func (m *Model) settingsContentHeight() int {
	controls := m.settingsControls()
	return controls[len(controls)-1].y + 3
}

func (m *Model) settingsMaxScroll() int {
	return max(0, m.settingsContentHeight()-m.footerTop())
}

func (m *Model) settingSelected(control settingControl) bool {
	switch control.row {
	case settingAgent:
		return control.key == m.agent
	case settingAppearance:
		return control.key == m.theme.Mode
	case settingAccent:
		return control.key == m.theme.Preset
	case settingLanguage:
		index := max(0, slices.Index(languages, m.interfacePrefs.Language))
		return control.key == languages[index]
	case settingShortcutBar:
		return control.key == string(m.interfacePrefs.ShortcutMode())
	}
	return false
}

func (m *Model) settingsView(f *frame, a appearance) {
	content := newFrame(f.width, m.settingsContentHeight(), a.base)
	b := m.settingsBounds()
	x, inner := b.x, b.w
	title := "Settings"
	subtitle := "Make Ashley feel like yours. Changes save automatically."
	if m.firstRun {
		title = "Welcome to Ashley"
		subtitle = "Choose your coding agent and appearance to get started."
	}
	content.put(x, 2, a.base.Bold(true).Render(m.tr(title)))
	content.text(rect{x, 4, inner, 2}, m.tr(subtitle), a.muted, 0)
	sections, controls := m.settingsLayout()
	for _, section := range sections {
		content.section(section.rect, m.tr(section.label), a)
	}
	surface := a.base.Background(lipgloss.Color(terminalColor(m.themePalette()["surface-lighten-1"])))
	for _, control := range controls {
		r := control.rect
		selected := m.settingSelected(control)
		focused := control.row == m.settingsRow && control.column == m.settingsColumn
		style := surface
		if control.row == settingLanguage || control.row == settingShortcutBar {
			style = a.base
		}
		if focused {
			style = style.Background(lipgloss.Color(blendColor(a.accent, a.bg, .18)))
		}
		if selected {
			style = style.Foreground(lipgloss.Color(a.accent)).Bold(true)
			if m.effectiveMode() == "light" {
				style = style.Foreground(lipgloss.Color(a.fg))
			}
		}
		if control.row == settingDone {
			style = a.base.Background(lipgloss.Color(a.accent)).Foreground(lipgloss.Color("#161616")).Bold(true)
		}
		content.fill(r, style)
		marker := " "
		if focused {
			marker = "›"
		}
		content.put(r.x, r.y, style.Render(marker))
		left := r.x + 2
		if control.row == settingAccent {
			preset := config.Presets()[control.column]
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
	for y := 1; y < m.footerTop(); y++ {
		source := y + m.screenScroll
		if source < content.height {
			f.put(0, y, content.row(source))
		}
	}
	if content.height > m.footerTop() {
		f.scrollbar(rect{b.x + b.w, 1, 1, m.footerTop() - 1}, content.height-2, m.screenScroll, a)
	}
}
func (m *Model) keepSettingVisible() {
	for _, control := range m.settingsControls() {
		if control.row == m.settingsRow && control.column == m.settingsColumn {
			if control.y-m.screenScroll < 2 {
				m.screenScroll = max(0, control.y-2)
			}
			if control.y+control.h-m.screenScroll > m.footerTop()-1 {
				m.screenScroll = control.y + control.h - m.footerTop() + 1
			}
		}
	}
	m.screenScroll = max(0, min(m.settingsMaxScroll(), m.screenScroll))
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
func (m *Model) focusSettings() {
	m.settingsRow, m.settingsColumn = 0, 0
	for i, key := range agents.Keys() {
		if key == m.agent {
			m.settingsColumn = i
			break
		}
	}
	m.keepSettingVisible()
}
func (m *Model) keepCreatorVisible() {
	if m.wizard == nil || m.wizard.stage != 0 {
		m.screenScroll = 0
		return
	}
	y := []int{8, 13, 18, 23}[m.wizard.field]
	bottom := y + 3
	if m.wizard.field == 3 {
		bottom = y + 5
	}
	if y-m.screenScroll < 2 {
		m.screenScroll = max(0, y-2)
	}
	if bottom-m.screenScroll > m.contentHeight()-3 {
		m.screenScroll = max(0, bottom-m.contentHeight()+3)
	}
}
