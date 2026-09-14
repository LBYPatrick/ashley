package tui

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/cellbuf"
)

type rect struct{ x, y, w, h int }

func (r rect) contains(x, y int) bool { return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h }

type frame struct {
	cells  *cellbuf.Buffer
	height int
	width  int
	base   lipgloss.Style
}

func newFrame(w, h int, base lipgloss.Style) *frame {
	f := &frame{width: max(1, w), height: max(1, h), base: base}
	f.cells = cellbuf.NewBuffer(f.width, f.height)
	f.fill(rect{0, 0, f.width, f.height}, base)
	return f
}
func (f *frame) put(x, y int, text string) {
	if y < 0 || y >= f.height || x >= f.width {
		return
	}
	if x < 0 {
		text = ansi.Cut(text, -x, ansi.StringWidth(text))
		x = 0
	}
	text = ansi.Truncate(text, f.width-x, "")
	width := ansi.StringWidth(text)
	// Compose terminal cells, never concatenate retained ANSI prefixes. Repeated
	// overlays otherwise multiply invisible styles and can stall an entire frame.
	cellbuf.SetContentRect(f.cells, text, cellbuf.Rect(x, y, width, 1))
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
func (f *frame) row(y int) string {
	_, line := cellbuf.RenderLine(f.cells, y)
	return line + strings.Repeat(" ", max(0, f.width-ansi.StringWidth(line)))
}
func (f *frame) String() string {
	rows := make([]string, f.height)
	for y := range rows {
		rows[y] = f.row(y)
	}
	return strings.Join(rows, "\n")
}

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
	if a.clear {
		for i := 0; i < r.h; i++ {
			glyph, style := "│", a.muted
			if i >= start/8 && i < (end+7)/8 {
				glyph, style = "┃", a.title
			}
			f.put(r.x, r.y+i, style.Render(strings.Repeat(glyph, r.w)))
		}
		return
	}
	bars := []rune("▁▂▃▄▅▆▇ ")
	for index := 0; index < r.h; index++ {
		style := a.base
		glyph := ' '
		if index >= start/8 && index < end/8 {
			style = a.base.Background(lipgloss.Color(blendColor(a.accent, a.bg, .25)))
		}
		if index == start/8 && bars[7-start%8] != ' ' {
			glyph = bars[7-start%8]
			style = a.base.Foreground(lipgloss.Color(blendColor(a.accent, a.bg, .25)))
		}
		if index == end/8 && bars[7-end%8] != ' ' {
			glyph = bars[7-end%8]
			style = a.base.Foreground(lipgloss.Color(blendColor(a.accent, a.bg, .25))).Reverse(true)
		}
		f.put(r.x, r.y+index, style.Render(strings.Repeat(string(glyph), r.w)))
	}
}

// clearBackground also handles reverse-video cursors from child widgets and
// ANSI backgrounds in captured output without losing their foreground colors.
func (f *frame) clearBackground() {
	for y := 0; y < f.height; y++ {
		for x := 0; x < f.width; x++ {
			c := f.cells.Cell(x, y)
			if c == nil {
				continue
			}
			c.Style.Bg = nil
			if c.Style.Attrs&cellbuf.ReverseAttr != 0 {
				c.Style.Attrs &^= cellbuf.ReverseAttr
				c.Style.UlStyle = cellbuf.SingleUnderline
			}
		}
	}
}
