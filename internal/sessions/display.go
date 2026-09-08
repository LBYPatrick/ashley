package sessions

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Preview uses tmux's rendered screen and history for live TUIs. Pipe-pane logs
// contain redraw instructions, not a sequence of printable transcript lines.
func (m Manager) Preview(s Session, n int, live bool) (string, error) {
	if live {
		content, err := m.command("capture-pane", "-p", "-t", s.TmuxSession, "-S", "-50")
		if err == nil {
			return tailDisplay(strings.TrimRight(content, "\n"), n), nil
		}
	}
	content, err := ReadLog(s, n)
	if err != nil {
		return "", err
	}
	return tailDisplay(DisplayLog(content), n), nil
}
func tailDisplay(content string, n int) string {
	lines := strings.Split(content, "\n")
	if n > 0 && len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// DisplayLog interprets common transcript redraws before presenting plain text.
// Escape sequences must never reach Ashley's own terminal renderer. Live agent
// screens use capture-pane above, which also handles full terminal state.
func DisplayLog(content string) string {
	type glyph struct {
		text  string
		width int
	}
	rows := map[int]map[int]glyph{}
	x, y, bottom, savedX, savedY := 0, 0, 0, 0, 0
	line := func() map[int]glyph {
		if rows[y] == nil {
			rows[y] = map[int]glyph{}
		}
		return rows[y]
	}
	erase := func(start, end int) {
		for col, g := range line() {
			if col < end && col+g.width > start {
				delete(rows[y], col)
			}
		}
	}
	p := ansi.GetParser()
	defer ansi.PutParser(p)
	var state byte
	for len(content) > 0 {
		seq, width, n, next := ansi.DecodeSequence(content, state, p)
		if n == 0 {
			break
		}
		content = content[n:]
		state = next
		if width > 0 {
			// A grapheme spans at most four cells. Check only overlaps here;
			// scanning the entire row for every character is quadratic on
			// long unwrapped agent output.
			current := line()
			for col := max(0, x-3); col < x+width; col++ {
				if g, ok := current[col]; ok && col+g.width > x {
					delete(current, col)
				}
			}
			current[x] = glyph{seq, width}
			x += width
			bottom = max(bottom, y)
			continue
		}
		param := func(i, def int) int { v, _ := p.Param(i, def); return v }
		move := max(1, param(0, 1))
		switch {
		case seq == "\r":
			x = 0
		case seq == "\n":
			y++
			x = 0
			bottom = max(bottom, y)
		case seq == "\b":
			x = max(0, x-1)
		case seq == "\t":
			x = (x/8 + 1) * 8
		case ansi.HasCsiPrefix(seq):
			switch p.Command() {
			case 'A':
				y = max(0, y-move)
			case 'B':
				y = min(bottom+1000, y+move)
			case 'C':
				x = min(16384, x+move)
			case 'D':
				x = max(0, x-move)
			case 'E':
				y = min(bottom+1000, y+move)
				x = 0
			case 'F':
				y = max(0, y-move)
				x = 0
			case 'G':
				x = min(16384, max(0, move-1))
			case 'H', 'f':
				y = min(bottom+1000, max(0, move-1))
				x = min(16384, max(0, param(1, 1)-1))
			case 'K':
				switch param(0, 0) {
				case 0:
					erase(x, 1<<30)
				case 1:
					erase(0, x+1)
				case 2:
					delete(rows, y)
				}
			case 'J':
				switch param(0, 0) {
				case 0:
					erase(x, 1<<30)
					for row := range rows {
						if row > y {
							delete(rows, row)
						}
					}
					bottom = y
				case 1:
					erase(0, x+1)
					for row := range rows {
						if row < y {
							delete(rows, row)
						}
					}
				case 2, 3:
					rows = map[int]map[int]glyph{}
					bottom = y
				}
			case 's':
				savedX, savedY = x, y
			case 'u':
				x, y = savedX, savedY
			}
		}
	}
	var out strings.Builder
	for row := 0; row <= bottom; row++ {
		end := 0
		for col, g := range rows[row] {
			end = max(end, col+g.width)
		}
		for col := 0; col < end; {
			if g, ok := rows[row][col]; ok {
				out.WriteString(g.text)
				col += g.width
			} else {
				out.WriteByte(' ')
				col++
			}
		}
		if row < bottom {
			out.WriteByte('\n')
		}
	}
	return strings.TrimRight(out.String(), "\n")
}
