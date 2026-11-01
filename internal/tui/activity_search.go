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
	y := l.input.y
	if l.input.h == 1 {
		y++
	}
	label = "^F " + label
	controls := []settingControl{{rect: rect{l.input.x, y, len(label), 1}, key: "scope", label: label}}
	if m.filter.Value() != "" {
		clear := "^U Clear"
		if l.input.w < len(label)+12 {
			clear = "^U"
		}
		controls = append(controls, settingControl{
			rect: rect{l.input.x + l.input.w - len(clear), y, len(clear), 1},
			key:  "clear", label: clear,
		})
	}
	return controls
}

func (m *Model) activitySearchView(f *frame, a appearance) {
	l := m.layout()
	placeholder := "Search " + m.screen + "…"
	if m.directoryOnly {
		placeholder = "Search directory…"
	}
	f.input(l.input, m.filter.Value(), placeholder, m.focus == "filter", a, m.filter.Position())
	for _, control := range m.activitySearchControls() {
		style := a.muted
		if control.key == "scope" {
			style = a.title
		}
		f.put(control.x, control.y, style.Render(ansi.Truncate(control.label, control.w, "…")))
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
