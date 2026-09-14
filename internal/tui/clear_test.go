package tui

import (
	"github.com/charmbracelet/x/cellbuf"
	"testing"
)

func assertClearFrame(t *testing.T, view string, w, h int) {
	t.Helper()
	b := cellbuf.NewBuffer(w, h)
	cellbuf.SetContent(b, view)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := b.Cell(x, y)
			if c != nil && (c.Style.Bg != nil || c.Style.Attrs&cellbuf.ReverseAttr != 0) {
				t.Fatalf("opaque cell at %d,%d: %+v", x, y, c.Style)
			}
		}
	}
}

func TestClearScreensAndComponents(t *testing.T) {
	trueColor(t)
	m := newModel(t)
	m.theme.Mode = "clear"
	for _, screen := range []string{"hub", "vibe", "settings", "sessions", "history", "skills.sh", "stats", "create", "help", "log", "sync"} {
		m.open(screen)
		assertClearFrame(t, m.View(), m.width, m.height)
		m.paletteOpen = true
		assertClearFrame(t, m.View(), m.width, m.height)
		m.paletteOpen = false
	}
	m.open("create")
	m.wizard = nil
	m.sizeCreatorEditor()
	assertClearFrame(t, m.View(), m.width, m.height)
	a := m.appearance()
	f := newFrame(20, 10, a.base)
	f.scrollbar(rect{0, 0, 1, 10}, 100, 90, a)
	f.put(2, 2, "\x1b[41;7moutput\x1b[0m")
	f.clearBackground()
	assertClearFrame(t, f.String(), 20, 10)
}
