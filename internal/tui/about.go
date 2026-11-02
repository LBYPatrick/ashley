package tui

import (
	"fmt"

	"github.com/LBYPatrick/ashley"
	"github.com/charmbracelet/bubbles/viewport"
)

type aboutOrigin struct {
	screen  string
	scroll  int
	preview viewport.Model
}

func aboutText() string {
	return fmt.Sprintf("Ashley\nVersion: %s\n\nAshley runs coding agents with shared skills.\nStart tasks, manage sessions, and resume conversations.\n\nProject\nhttps://github.com/LBYPatrick/ashley\n\nLicense: MIT\nCopyright (c) 2025 Ashley Contributors", ashley.Version())
}

func (m *Model) openAbout() {
	if m.screen == "about" {
		return
	}
	m.aboutOrigin = &aboutOrigin{screen: m.screen, scroll: m.screenScroll, preview: m.preview}
	m.screen, m.screenScroll = "about", 0
}

func (m *Model) closeAbout() {
	if m.aboutOrigin == nil {
		m.open("hub")
		return
	}
	origin := m.aboutOrigin
	m.aboutOrigin = nil
	m.screen, m.screenScroll, m.preview = origin.screen, origin.scroll, origin.preview
	if m.screen == "log" || m.screen == "help" || m.screen == "create-preview" {
		m.sizeLogPreview()
	} else if m.isActivity() || m.screen == "vibe" || m.screen == "hub" || m.screen == "skills.sh" {
		offset := m.preview.YOffset
		m.setPreviewContent()
		m.preview.SetYOffset(offset)
	}
}

func (m *Model) aboutMaxScroll() int {
	r := m.readingRect()
	return max(0, wrappedLines(aboutText(), r.w)-r.h)
}

func (m *Model) scrollAbout(delta int) {
	m.screenScroll = max(0, min(m.aboutMaxScroll(), m.screenScroll+delta))
}

func (m *Model) aboutKey(key string) {
	switch key {
	case "esc":
		m.closeAbout()
	case "up", "k":
		m.scrollAbout(-1)
	case "down", "j":
		m.scrollAbout(1)
	case "pgup":
		m.scrollAbout(-m.readingRect().h)
	case "pgdown":
		m.scrollAbout(m.readingRect().h)
	case "home":
		m.screenScroll = 0
	case "end":
		m.screenScroll = m.aboutMaxScroll()
	}
}

func (m *Model) aboutView(f *frame, a appearance) {
	f.richText(m.readingRect(), aboutText(), a, m.screenScroll)
}
