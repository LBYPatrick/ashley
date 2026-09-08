package tui

import (
	"github.com/charmbracelet/x/ansi"
	"strings"
)

func (m *Model) readingRect() rect {
	width := min(100, max(12, m.width-8))
	return rect{(max(20, m.width) - width) / 2, 3, width, max(1, m.height-6)}
}
func wrappedLines(text string, width int) int {
	return len(strings.Split(ansi.Wrap(text, max(1, width), ""), "\n"))
}

// Preserve every value while distinguishing headings, labels, and actions.
func (f *frame) richText(r rect, text string, a appearance, offset int) {
	lines := strings.Split(ansi.Wrap(text, max(1, r.w), ""), "\n")
	offset = max(0, min(offset, max(0, len(lines)-r.h)))
	for i := offset; i < min(len(lines), offset+r.h); i++ {
		style := a.base
		line := lines[i]
		if i == 0 {
			style = style.Bold(true)
		}
		switch line {
		case "Overview", "Workflow", "Destination", "Result", "Keyboard shortcuts":
			style = style.Bold(true)
		}
		if strings.HasPrefix(line, "✓") {
			style = a.title
		}
		f.put(r.x, r.y+i-offset, style.Render(ansi.Truncate(line, r.w, "")))
	}
	if len(lines) > r.h {
		f.scrollbar(rect{r.x + r.w, r.y, 1, r.h}, len(lines), offset, a)
	}
}
func screenName(screen string) string {
	names := map[string]string{"hub": "Home", "vibe": "Skills", "sessions": "Sessions", "history": "History", "stats": "Analytics", "create": "Create skill", "create-preview": "Skill preview", "settings": "Settings", "sync": "Sync", "log": "Session log", "help": "Keyboard shortcuts"}
	return names[screen]
}
