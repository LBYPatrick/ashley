package tui

import (
	"github.com/charmbracelet/x/ansi"
	"strings"
)

func (m *Model) sessionActionControls() []settingControl {
	if m.screen != "sessions" || m.compactDetail {
		return nil
	}
	l := m.layout()
	labels := []string{"X Kill all", "D Clean dead"}
	if l.list.w < 27 {
		labels = []string{"X All", "D Dead"}
	}
	var controls []settingControl
	x := l.list.x
	for i, key := range []string{"X", "cleanup"} {
		width := ansi.StringWidth(labels[i]) + 2
		controls = append(controls, settingControl{rect: rect{x, l.list.y - 1, width, 1}, key: key, label: labels[i]})
		x += width + 1
	}
	return controls
}

// Share the exact visible rectangles between rendering, wrapping, and scrolling.
func (m *Model) sessionPanels() (detail, log, body rect) {
	l := m.layout()
	lines := strings.Split(ansi.Wrap(m.detailText(), max(1, l.detail.w), ""), "\n")
	reserve := max(7, min(12, l.detail.h/2))
	detail = l.detail
	detail.h = min(len(lines), max(1, l.detail.h-reserve))
	log = rect{l.detail.x, l.detail.y + detail.h + 1, l.detail.w, max(3, l.detail.h-detail.h-1)}
	body = rect{log.x, log.y + 3, log.w, max(1, log.h-3)}
	return
}
func (m *Model) maxLogOffset() int {
	_, _, body := m.sessionPanels()
	lines := strings.Split(ansi.Wrap(m.sessionLog, body.w, ""), "\n")
	return max(0, len(lines)-body.h)
}

func (m *Model) sizeLogPreview() {
	offset := m.preview.YOffset
	r := m.readingRect()
	m.preview.Width = r.w
	m.preview.Height = r.h
	m.preview.SetContent(ansi.Wrap(m.logContent, m.preview.Width, ""))
	m.preview.SetYOffset(offset)
}
