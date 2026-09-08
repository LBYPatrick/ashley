package tui

import (
	"github.com/charmbracelet/x/ansi"
	"strings"
)

// Share the exact visible rectangles between rendering, wrapping, and scrolling.
func (m *Model) sessionPanels() (detail, log, body rect) {
	l := m.layout()
	lines := strings.Split(ansi.Wrap(m.detailText(), max(1, l.detail.w), ""), "\n")
	reserve := max(7, min(12, l.detail.h/2))
	detail = l.detail
	detail.h = min(len(lines), max(1, l.detail.h-reserve))
	log = rect{l.right.x + 4, l.detail.y + detail.h + 1, max(4, l.right.w-8), max(4, l.right.y+l.right.h-l.detail.y-detail.h-4)}
	body = rect{log.x + 3, log.y + 3, max(1, log.w-6), max(1, log.h-4)}
	return
}
func (m *Model) maxLogOffset() int {
	_, _, body := m.sessionPanels()
	lines := strings.Split(ansi.Wrap(m.sessionLog, body.w, ""), "\n")
	return max(0, len(lines)-body.h)
}

func (m *Model) sizeLogPreview() {
	offset := m.preview.YOffset
	m.preview.Width = max(1, m.width-8)
	m.preview.Height = max(1, m.height-7)
	m.preview.SetContent(ansi.Wrap(m.logContent, m.preview.Width, ""))
	m.preview.SetYOffset(offset)
}
