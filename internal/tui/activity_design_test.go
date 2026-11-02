package tui

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/LBYPatrick/ashley/internal/config"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/cellbuf"
)

func TestActivitySidebarHierarchyAndBalance(t *testing.T) {
	for _, width := range []int{80, 100, 120, 160} {
		m := activitySearchModel(t)
		m.width, m.height = width, 30
		m.open("sessions")
		l := m.layout()
		if l.left.w < 30 || l.left.w > 52 || l.right.w < 30 || l.input.h != 1 {
			t.Fatal("sidebar lacks room for titles, search, or details", width, l)
		}
		view := m.View()
		lines := strings.Split(ansi.Strip(view), "\n")
		for row := range 2 {
			y := l.list.y + row*m.rowHeight()
			gap := ansi.Cut(lines[y+2], l.list.x, l.list.x+l.list.w)
			if strings.TrimSpace(gap) != "" {
				t.Fatal("task rows have no separation", gap)
			}
		}
		if strings.Contains(ansi.Cut(lines[l.list.y+1], l.list.x, l.list.x+l.list.w), "feat") {
			t.Fatal("secondary row repeats metadata instead of showing the directory")
		}
		m.open("history")
		if m.layout().left.w != l.left.w {
			t.Fatal("switching tabs changes panel balance")
		}
	}
}

func TestActivitySelectedRowsAndActionsContrast(t *testing.T) {
	trueColor(t)
	for _, monochrome := range []bool{false, true} {
		for _, mode := range []string{"light", "dark"} {
			for _, preset := range config.Presets() {
				t.Run(fmt.Sprintf("%s-%s/NO_COLOR=%t", mode, preset.Key, monochrome), func(t *testing.T) {
					t.Setenv("NO_COLOR", "1")
					if !monochrome {
						if err := os.Unsetenv("NO_COLOR"); err != nil {
							t.Fatal(err)
						}
					}
					m := activitySearchModel(t)
					m.theme = config.Theme{Mode: mode, Preset: preset.Key}
					m.open("sessions")
					l := m.layout()
					buffer := cellbuf.NewBuffer(m.width, m.height)
					cellbuf.SetContent(buffer, m.View())
					for _, point := range [][2]int{{l.list.x + 4, l.list.y}, {l.list.x + 4, l.list.y + 1}, {l.list.x + l.list.w - 1, l.list.y}} {
						cell := buffer.Cell(point[0], point[1])
						if ratio := renderedContrast(cell.Style.Fg, cell.Style.Bg); ratio < 4.5 {
							t.Fatalf("selected task text has %.2f:1 contrast", ratio)
						}
					}
					m.openSessionActions()
					cellbuf.SetContent(buffer, m.View())
					r := m.paletteBounds()
					if r.h > 9 {
						t.Fatal("two session actions use excessive vertical space")
					}
					for y := r.y; y < r.y+r.h; y++ {
						for x := r.x; x < r.x+r.w; x++ {
							cell := buffer.Cell(x, y)
							if cell != nil && cell.Rune != 0 && cell.Rune != ' ' {
								if ratio := renderedContrast(cell.Style.Fg, cell.Style.Bg); ratio < 4.5 {
									t.Fatalf("session action text or border has %.2f:1 contrast", ratio)
								}
							}
						}
					}
					exportRegressionView(t, fmt.Sprintf("activity-actions-%s-%s", mode, preset.Key), m.View())
				})
			}
		}
	}
}
