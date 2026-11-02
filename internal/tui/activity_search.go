package tui

import "github.com/charmbracelet/x/ansi"

func (m *Model) isActivity() bool {
	return m.screen == "sessions" || m.screen == "history"
}

func (m *Model) applySearch() {
	m.offset, m.cursor = 0, 0
	m.refresh()
}

func (m *Model) toggleSearchScope() {
	m.directoryOnly = !m.directoryOnly
	m.applySearch()
}

func (m *Model) activityTabControls() []settingControl {
	if !m.isActivity() || m.compactDetail {
		return nil
	}
	l := m.layout()
	labels, step := []string{"Sessions", "History"}, 12
	if l.left.w < 23 {
		labels, step = []string{"Sess", "Hist"}, 7
	}
	var controls []settingControl
	for i, screen := range []string{"sessions", "history"} {
		controls = append(controls, settingControl{
			rect: rect{l.left.x + i*step, l.left.y, len(labels[i]) + 2, 1},
			key:  screen, label: labels[i],
		})
	}
	return controls
}

// Scope and clear controls share their visible rectangles with mouse handling.
func (m *Model) activitySearchControls() []settingControl {
	if !m.isActivity() || m.compactDetail {
		return nil
	}
	l := m.layout()
	label := "All fields"
	if m.directoryOnly {
		label = "Directory"
	}
	if l.input.w < 30 {
		label = "All"
		if m.directoryOnly {
			label = "Dir"
		}
	}
	label = "[" + label + "]"
	x := l.input.x + l.input.w
	var clear *settingControl
	if m.filter.Value() != "" {
		x -= 3
		clear = &settingControl{rect: rect{x, l.input.y, 3, 1}, key: "clear", label: "[x]"}
		x--
	}
	x -= len(label)
	controls := []settingControl{{rect: rect{x, l.input.y, len(label), 1}, key: "scope", label: label}}
	if clear != nil {
		controls = append(controls, *clear)
	}
	return controls
}

func (m *Model) activitySearchView(f *frame, a appearance) {
	l := m.layout()
	controls := m.activitySearchControls()
	field := l.input
	field.w = max(1, controls[0].x-field.x-1)
	placeholder := "Search " + m.screen + "…"
	if m.directoryOnly {
		placeholder = "Search directory…"
	}
	if field.w < 21 {
		placeholder = "Search…"
	}
	f.fill(l.input, a.panel)
	f.input(field, m.filter.Value(), placeholder, m.focus == "filter", a, m.filter.Position())
	for _, control := range controls {
		f.put(control.x, control.y, a.panel.Render(control.label))
	}
}

func directoryLabel(path string, width int) string {
	if path == "" {
		path = "Directory unavailable"
	}
	width = max(1, width)
	if pathWidth := ansi.StringWidth(path); pathWidth > width {
		return "…" + ansi.Cut(path, pathWidth-width+1, pathWidth)
	}
	return path
}
