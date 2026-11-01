package tui

import (
	"math"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Dialog text and borders use the stronger black/white contrast on the actual
// panel background. Inverting that pair also makes focus visible without tint.
func confirmationStyles(a appearance) (body, focused lipgloss.Style) {
	background := a.panel.GetBackground()
	r, g, b, _ := background.RGBA()
	linear := func(channel uint32) float64 {
		value := float64(channel) / 65535
		if value <= .04045 {
			return value / 12.92
		}
		return math.Pow((value+.055)/1.055, 2.4)
	}
	luminance := .2126*linear(r) + .7152*linear(g) + .0722*linear(b)
	foreground := lipgloss.Color("#FFFFFF")
	if (luminance+.05)/.05 >= 1.05/(luminance+.05) {
		foreground = lipgloss.Color("#000000")
	}
	body = lipgloss.NewStyle().Foreground(foreground).Background(background)
	focused = lipgloss.NewStyle().Foreground(background).Background(foreground).Bold(true)
	return body, focused
}

func (m *Model) confirmBounds() rect {
	w := min(62, max(20, m.width)-4)
	h := min(max(8, m.height)-2, wrappedLines(m.confirm.body, w-4)+7)
	return rect{(max(20, m.width) - w) / 2, (max(8, m.height) - h) / 2, w, h}
}

func (m *Model) confirmButtons() []settingControl {
	r := m.confirmBounds()
	stacked := r.w-4 < 23
	y, x := r.y+r.h-2, r.x+2
	if stacked {
		y--
	}
	var controls []settingControl
	for i, label := range []string{"Cancel", "Confirm"} {
		width := len(label) + 4
		text := "[ " + label + " ]"
		if (i == 1) == m.confirm.proceed {
			text = "> " + label + " <"
		}
		controls = append(controls, settingControl{
			rect: rect{x, y, width, 1}, key: label, label: text,
		})
		if stacked {
			y++
		} else {
			x += width + 2
		}
	}
	return controls
}

func (m *Model) confirmBodyRect() rect {
	r := m.confirmBounds()
	top, gap := r.y+3, 1
	if r.h < 9 {
		top, gap = r.y+2, 0
	}
	buttons := m.confirmButtons()
	return rect{r.x + 2, top, r.w - 4, max(1, buttons[0].y-top-gap)}
}

func (m *Model) maxConfirmationScroll() int {
	r := m.confirmBodyRect()
	return max(0, wrappedLines(m.confirm.body, r.w)-r.h)
}

func (m *Model) scrollConfirmation(delta int) {
	m.confirm.scroll = max(0, min(m.maxConfirmationScroll(), m.confirm.scroll+delta))
}

func (m *Model) confirmView(f *frame, a appearance) {
	r := m.confirmBounds()
	body, focused := confirmationStyles(a)
	f.fill(r, body)
	f.box(r, body)
	f.put(r.x+2, r.y+1, body.Bold(true).Render(ansi.Truncate(m.confirm.title, r.w-4, "…")))
	content := m.confirmBodyRect()
	offset := min(m.confirm.scroll, m.maxConfirmationScroll())
	f.text(content, m.confirm.body, body, offset)
	if m.maxConfirmationScroll() > 0 {
		indicator := "↕"
		if offset == 0 {
			indicator = "↓"
		} else if offset == m.maxConfirmationScroll() {
			indicator = "↑"
		}
		f.put(content.x+content.w, content.y, body.Render(indicator))
	}
	for i, control := range m.confirmButtons() {
		style := body
		if (i == 1) == m.confirm.proceed {
			style = focused
		}
		f.put(control.x, control.y, style.Render(control.label))
	}
}
