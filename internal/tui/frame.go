package tui

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type rect struct{ x, y, w, h int }

func (r rect) contains(x, y int) bool { return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h }

type frame struct {
	rows  []string
	width int
	base  lipgloss.Style
}

func newFrame(w, h int, base lipgloss.Style) *frame {
	f := &frame{width: max(1, w), base: base}
	for range max(1, h) {
		f.rows = append(f.rows, base.Render(strings.Repeat(" ", f.width)))
	}
	return f
}
func (f *frame) put(x, y int, text string) {
	if y < 0 || y >= len(f.rows) || x >= f.width {
		return
	}
	if x < 0 {
		text = ansi.Cut(text, -x, ansi.StringWidth(text))
		x = 0
	}
	text = ansi.Truncate(text, f.width-x, "")
	width := ansi.StringWidth(text)
	f.rows[y] = ansi.Cut(f.rows[y], 0, x) + text + ansi.Cut(f.rows[y], x+width, f.width)
}
func (f *frame) fill(r rect, style lipgloss.Style) {
	for y := r.y; y < r.y+r.h; y++ {
		f.put(r.x, y, style.Render(strings.Repeat(" ", max(0, r.w))))
	}
}
func (f *frame) box(r rect, style lipgloss.Style) {
	if r.w < 2 || r.h < 2 {
		return
	}
	f.put(r.x, r.y, style.Render("╭"+strings.Repeat("─", r.w-2)+"╮"))
	for y := r.y + 1; y < r.y+r.h-1; y++ {
		f.put(r.x, y, style.Render("│"))
		f.put(r.x+r.w-1, y, style.Render("│"))
	}
	f.put(r.x, r.y+r.h-1, style.Render("╰"+strings.Repeat("─", r.w-2)+"╯"))
}
func (f *frame) text(r rect, text string, style lipgloss.Style, offset int) {
	if r.w < 1 || r.h < 1 {
		return
	}
	lines := strings.Split(ansi.Wrap(text, r.w, ""), "\n")
	for index := max(0, offset); index < min(len(lines), max(0, offset)+r.h); index++ {
		f.put(r.x, r.y+index-max(0, offset), style.Render(lines[index]))
	}
}
func (f *frame) String() string { return strings.Join(f.rows, "\n") }

// scrollbar reproduces Textual's eighth-cell thumb sizing and end caps.
func (f *frame) scrollbar(r rect, virtual, position int, a appearance) {
	if r.h < 1 || virtual <= r.h {
		return
	}
	thumb := max(1.0, float64(r.h*r.h)/float64(virtual))
	offset := float64(r.h) - thumb
	offset *= float64(position) / float64(virtual-r.h)
	start := int(offset * 8)
	end := start + int(math.Ceil(thumb*8))
	bars := []rune("▁▂▃▄▅▆▇ ")
	for index := 0; index < r.h; index++ {
		style := a.base
		glyph := ' '
		if index >= start/8 && index < end/8 {
			style = a.base.Background(lipgloss.Color(blendColor(a.accent, a.bg, .005)))
		}
		if index == start/8 && bars[7-start%8] != ' ' {
			glyph = bars[7-start%8]
			style = a.base.Foreground(lipgloss.Color(blendColor(a.accent, a.bg, .005)))
		}
		if index == end/8 && bars[7-end%8] != ' ' {
			glyph = bars[7-end%8]
			style = a.base.Foreground(lipgloss.Color(blendColor(a.accent, a.bg, .005))).Reverse(true)
		}
		f.put(r.x, r.y+index, style.Render(strings.Repeat(string(glyph), r.w)))
	}
}
