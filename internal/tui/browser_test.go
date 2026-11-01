package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/LBYPatrick/ashley/internal/sessions"
	"github.com/charmbracelet/x/ansi"
)

func TestSessionRowsKeepStatusAndElapsedVisible(t *testing.T) {
	for _, size := range [][2]int{{50, 16}, {50, 24}, {100, 16}, {120, 40}} {
		m := newModel(t)
		m.open("sessions")
		m.width, m.height = size[0], size[1]
		m.sessionRows = []sessions.Session{
			{ID: "running", Name: strings.Repeat("Long task title ", 20), Skill: "feat", StartedAt: time.Now().Add(-2 * time.Hour).Format(time.RFC3339Nano)},
			{ID: "finished", Name: "完成した作業", Skill: "coding", StartedAt: "invalid"},
			{ID: "checking", Name: "Checking status", Skill: "coding", StartedAt: "invalid"},
		}
		m.sessionAlive = map[string]bool{"running": true, "finished": false}
		for _, cursor := range []int{0, 1, 2} {
			m.cursor = cursor
			view := m.View()
			assertFrame(t, view, size[0], size[1])
			lines := strings.Split(ansi.Strip(view), "\n")
			list := m.layout().list
			start := max(0, cursor-m.visibleRows()+1)
			for i, expected := range []struct{ symbol, elapsed string }{{"●", "2h 0m"}, {"○", "?"}, {"?", "?"}} {
				if i < start || i >= start+m.visibleRows() {
					continue
				}
				line := ansi.Cut(lines[list.y+(i-start)*m.rowHeight()], list.x, list.x+list.w)
				if ansi.Cut(line, 2, 3) != expected.symbol || !strings.HasSuffix(line, expected.elapsed) {
					t.Fatalf("size %v cursor %d row %d hides status or time: %q", size, cursor, i, line)
				}
				selected := strings.HasPrefix(line, "› ")
				if selected != (cursor == i) {
					t.Fatalf("selection does not match cursor: %q", line)
				}
			}
		}
	}
}

func TestActivityRowsFitBetweenSearchAndFooter(t *testing.T) {
	for _, screen := range []string{"sessions", "history"} {
		m := newModel(t)
		m.open(screen)
		for height := 8; height <= 24; height++ {
			m.height = height
			if rows, available := m.rowHeight(), m.layout().list.h; rows > available {
				t.Fatalf("%s at height %d draws %d lines into %d result lines", screen, height, rows, available)
			}
		}
	}
}
