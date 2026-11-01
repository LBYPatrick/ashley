package tui

import (
	"fmt"
	"image/color"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/LBYPatrick/ashley/internal/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/cellbuf"
)

func renderedContrast(a, b color.Color) float64 {
	luminance := func(c color.Color) float64 {
		converted := color.NRGBAModel.Convert(c).(color.NRGBA)
		channels := []uint8{converted.R, converted.G, converted.B}
		weights := []float64{.2126, .7152, .0722}
		var value float64
		for i, channel := range channels {
			component := float64(channel) / 255
			if component <= .04045 {
				component /= 12.92
			} else {
				component = math.Pow((component+.055)/1.055, 2.4)
			}
			value += component * weights[i]
		}
		return value
	}
	x, y := luminance(a), luminance(b)
	return (max(x, y) + .05) / (min(x, y) + .05)
}

func assertConfirmationContrast(t *testing.T, m *Model, f *frame) {
	t.Helper()
	r, body := m.confirmBounds(), m.confirmBodyRect()
	bodyBackground := f.cells.Cell(body.x, body.y).Style.Bg
	if bodyBackground == nil {
		t.Fatal("dialog body does not set an explicit background")
	}
	for y := r.y; y < r.y+r.h; y++ {
		for x := r.x; x < r.x+r.w; x++ {
			cell := f.cells.Cell(x, y)
			if cell == nil || cell.Rune == 0 || cell.Rune == ' ' {
				continue
			}
			if cell.Style.Fg == nil || cell.Style.Bg == nil {
				t.Fatalf("dialog cell %d,%d lacks explicit foreground or background", x, y)
			}
			if ratio := renderedContrast(cell.Style.Fg, cell.Style.Bg); ratio < 4.5 {
				t.Fatalf("dialog glyph %q at %d,%d has %.2f:1 contrast", cell.String(), x, y, ratio)
			}
		}
	}
	for _, point := range [][2]int{{r.x, r.y}, {r.x + 2, r.y + 1}} {
		cell := f.cells.Cell(point[0], point[1])
		if renderedContrast(cell.Style.Bg, bodyBackground) != 1 {
			t.Fatal("title or border has a different background from the dialog")
		}
		if ratio := renderedContrast(cell.Style.Fg, bodyBackground); ratio < 4.5 {
			t.Fatalf("title or border lacks contrast against the dialog: %.2f:1", ratio)
		}
	}
	for i, control := range m.confirmButtons() {
		cell := f.cells.Cell(control.x, control.y)
		if (i == 1) == m.confirm.proceed {
			if !strings.HasPrefix(control.label, "> ") || !strings.HasSuffix(control.label, " <") {
				t.Fatal("focused button has no text marker")
			}
			if ratio := renderedContrast(cell.Style.Bg, bodyBackground); ratio < 3 {
				t.Fatalf("focus background has %.2f:1 contrast against the dialog", ratio)
			}
		} else if !strings.HasPrefix(control.label, "[ ") {
			t.Fatal("unfocused button has no visible boundary")
		}
	}
	for x := 0; x < m.width; x++ {
		cell := f.cells.Cell(x, m.height-1)
		if cell == nil || cell.Rune == 0 || cell.Rune == ' ' {
			continue
		}
		if cell.Style.Fg == nil || cell.Style.Bg == nil {
			t.Fatal("confirmation footer lacks explicit text or background color")
		}
		if ratio := renderedContrast(cell.Style.Fg, cell.Style.Bg); ratio < 4.5 {
			t.Fatalf("confirmation footer glyph %q has %.2f:1 contrast", cell.String(), ratio)
		}
	}
}

func TestConfirmationRenderedContrastForEveryTheme(t *testing.T) {
	trueColor(t)
	for _, monochrome := range []bool{false, true} {
		for _, mode := range []string{"dark", "light"} {
			for _, preset := range config.Presets() {
				t.Run(fmt.Sprintf("%s-%s/NO_COLOR=%t", mode, preset.Key, monochrome), func(t *testing.T) {
					if monochrome {
						t.Setenv("NO_COLOR", "1")
					} else {
						t.Setenv("NO_COLOR", "")
						// NO_COLOR is active whenever it exists, including an empty value.
						if err := os.Unsetenv("NO_COLOR"); err != nil {
							t.Fatal(err)
						}
					}
					m := newModel(t)
					m.theme.Mode, m.theme.Preset = mode, preset.Key
					m.open("sessions")
					m.requestConfirmation("X")
					for _, proceed := range []bool{false, true} {
						m.confirm.proceed = proceed
						buffer := cellbuf.NewBuffer(m.width, m.height)
						cellbuf.SetContent(buffer, m.View())
						assertConfirmationContrast(t, m, &frame{cells: buffer})
					}
				})
			}
		}
	}
}

func TestConfirmationContrastForCustomPanelColors(t *testing.T) {
	trueColor(t)
	m := newModel(t)
	m.open("sessions")
	m.requestConfirmation("X")
	for _, panel := range []string{"#000000", "#FFFFFF", "#777777", "#00FF00", "#FF00FF", "#0000FF"} {
		for _, proceed := range []bool{false, true} {
			m.confirm.proceed = proceed
			a := m.appearance()
			a.panel = a.panel.Background(lipgloss.Color(panel))
			f := newFrame(m.width, m.height, a.base)
			m.confirmView(f, a)
			assertConfirmationContrast(t, m, f)
		}
	}
}

func TestConfirmationButtonsWarningScrollingAndModalPriority(t *testing.T) {
	trueColor(t)
	for _, size := range [][2]int{{20, 8}, {30, 10}, {50, 16}, {100, 30}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m := activitySearchModel(t)
			m.open("sessions")
			m.width, m.height = size[0], size[1]
			key(m, "X")
			r, body := m.confirmBounds(), m.confirmBodyRect()
			for _, control := range m.confirmButtons() {
				if !r.contains(control.x, control.y) || !r.contains(control.x+control.w-1, control.y) || control.y < body.y+body.h {
					t.Fatal("dialog button escapes the dialog or covers warning text", control, r, body)
				}
				if !strings.Contains(ansi.Strip(m.View()), control.label) {
					t.Fatal("dialog button is not completely visible", control)
				}
			}
			key(m, "N")
			if m.confirm == nil || m.rename != nil {
				t.Fatal("rename opened over the destructive confirmation")
			}
			if m.maxConfirmationScroll() > 0 {
				if !strings.Contains(ansi.Strip(m.View()), "↑↓ Read") {
					t.Fatal("clipped warning has no scroll instructions")
				}
				for range 100 {
					m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
				}
				if m.confirm.scroll != m.maxConfirmationScroll() || !strings.Contains(ansi.Strip(m.View()), "work.") {
					t.Fatal("scroll cannot reach the warning tail", m.confirm.scroll, m.View())
				}
				m.Update(tea.MouseMsg{X: body.x, Y: body.y, Button: tea.MouseButtonWheelUp})
				if m.confirm.scroll >= m.maxConfirmationScroll() {
					t.Fatal("mouse wheel did not scroll the warning")
				}
			}
			for _, mode := range []string{"dark", "light"} {
				m.theme.Mode = mode
				exportRegressionView(t, fmt.Sprintf("kill-confirm-default-%s-%dx%d", mode, size[0], size[1]), m.View())
			}
			key(m, "tab")
			if !m.confirm.proceed {
				t.Fatal("keyboard cannot select the destructive action")
			}
			assertFrame(t, m.View(), size[0], size[1])
			for _, mode := range []string{"dark", "light"} {
				m.theme.Mode = mode
				exportRegressionView(t, fmt.Sprintf("kill-confirm-%s-%dx%d", mode, size[0], size[1]), m.View())
			}
			cancel := m.confirmButtons()[0]
			m.Update(tea.MouseMsg{X: cancel.x + cancel.w, Y: cancel.y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
			if m.confirm == nil {
				t.Fatal("mouse target extends past the visible Cancel button")
			}
			m.Update(tea.MouseMsg{X: cancel.x + cancel.w - 1, Y: cancel.y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
			if m.confirm != nil {
				t.Fatal("visible Cancel button does not match its mouse target")
			}
			if rows, err := m.manager().All(); err != nil || len(rows) != 2 {
				t.Fatal("cancel removed saved sessions", rows, err)
			}
		})
	}
}
