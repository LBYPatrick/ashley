package tui

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/LBYPatrick/ashley/internal/config"
	"github.com/LBYPatrick/ashley/internal/history"
	"github.com/LBYPatrick/ashley/internal/sessions"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestActivityFooterShowsOnlyPrimaryActions(t *testing.T) {
	for _, screen := range []string{"sessions", "history"} {
		m := newModel(t)
		m.open(screen)
		m.width, m.height = 160, 30
		rows := m.footerRows()
		if len(rows) != 1 || !strings.Contains(rows[0], "/ Search") ||
			!strings.Contains(rows[0], "? All keys") || strings.Contains(rows[0], "d Delete") {
			t.Fatalf("%s footer should show only primary actions and help: %v", screen, rows)
		}
	}
}

func TestStandardActivityFooterShowsSessionShortcuts(t *testing.T) {
	m := newModel(t)
	m.open("activity")
	for _, language := range languages[1:] {
		m.language = language
		for _, width := range []int{20, 30, 45, 70, 100} {
			m.width = width
			rows := m.footerRows()
			if len(rows) != 1 || ansi.StringWidth(rows[0]) > width-4 {
				t.Fatalf("%s at width %d: %v", language, width, rows)
			}
			parts := strings.Split(rows[0], "  ")
			for _, binding := range []string{"N Rename", "K Stop", "X Stop all", "? All keys"} {
				key, _, _ := strings.Cut(binding, " ")
				want := m.localizedBinding(binding)
				if width < 70 && slices.Contains(parts, key) {
					continue
				}
				if !slices.Contains(parts, want) {
					t.Errorf("%s at width %d is missing %q: %v", language, width, want, rows)
				}
			}
		}
	}
}

func TestActivityFooterReservesResultsAndStatus(t *testing.T) {
	for _, screen := range []string{"sessions", "history"} {
		for _, width := range []int{20, 30, 50, 80, 100} {
			for height := 8; height <= 30; height++ {
				t.Run(fmt.Sprintf("%s/%dx%d", screen, width, height), func(t *testing.T) {
					m := newModel(t)
					m.open(screen)
					m.width, m.height = width, height
					m.sessionRows = []sessions.Session{{ID: "one", Name: "Job"}, {ID: "two", Name: "Job"}}
					m.historyRows = []history.Invocation{{ID: 1, Name: "Job"}, {ID: 2, Name: "Job"}}
					m.sessionAlive = map[string]bool{"one": false, "two": false}
					m.total = 100
					m.status = "Ready"
					view := m.View()
					assertFrame(t, view, width, height)
					rows := strings.Split(ansi.Strip(view), "\n")
					list := m.layout().list
					if !strings.Contains(rows[list.y], "Job") {
						t.Fatalf("footer or page hint covers the selected result:\n%s", ansi.Strip(view))
					}
					if list.y+list.h > m.footerTop()-1 || !strings.Contains(rows[m.footerTop()-1], "Ready") {
						t.Fatal("results or shortcuts cover the status row", list, m.footerTop())
					}
					if len(m.footerRows()) == 1 && !strings.Contains(rows[height-1], "? All keys") {
						t.Fatal("small terminal has no route to the complete shortcut list")
					}
				})
			}
		}
	}
}

func TestSmallActivityFooterOpensCompleteContextualHelp(t *testing.T) {
	for _, screen := range []string{"sessions", "history"} {
		m := newModel(t)
		m.open(screen)
		m.width, m.height = 20, 8
		key(m, "?")
		want := []string{"N Rename", "d Delete", "^U Clear", "v Details"}
		if screen == "sessions" {
			want = append(want, "K Stop", "X Stop all", "l Log", "D Clean finished")
		} else {
			want = append(want, "n/p Pages")
		}
		if m.screen != "help" {
			t.Fatal("shortcut help did not open")
		}
		for _, binding := range want {
			if !strings.Contains(strings.Join(m.help.context, "\n"), binding) {
				t.Errorf("%s help hides %q", screen, binding)
			}
		}
		text := strings.Join(m.help.context, "\n")
		key(m, "?")
		if strings.Join(m.help.context, "\n") != text {
			t.Fatal("opening help again replaced the original screen shortcuts")
		}
		m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		if m.help.offset == 0 {
			t.Fatal("small terminal cannot scroll the complete shortcut list")
		}
		key(m, "esc")
		if m.screen != screen {
			t.Fatal("help did not return to its original screen")
		}
	}
}

func TestActivityInputAndConfirmationFootersShowActiveControls(t *testing.T) {
	for _, width := range []int{20, 50, 100} {
		m := newModel(t)
		m.open("sessions")
		m.width, m.height = width, 8
		key(m, "/")
		footer := strings.Join(m.footerRows(), "\n")
		if !strings.Contains(footer, "esc") || !(strings.Contains(footer, "enter") || strings.Contains(footer, "↵")) {
			t.Fatalf("input footer hides how to return to results: %q", footer)
		}
		key(m, "esc")
		key(m, "X")
		footer = strings.Join(m.footerRows(), "\n")
		for _, binding := range []string{"↑↓ Read", "tab", "esc"} {
			if !strings.Contains(footer, binding) {
				t.Errorf("confirmation footer hides %q: %q", binding, footer)
			}
		}
		if !(strings.Contains(footer, "enter") || strings.Contains(footer, "↵")) {
			t.Fatalf("confirmation footer hides Accept: %q", footer)
		}
		for _, row := range m.footerRows() {
			if ansi.StringWidth(row) > width-4 {
				t.Fatalf("modal footer would be truncated at width %d: %q", width, row)
			}
		}
		assertFrame(t, m.View(), width, m.height)
	}
}

func TestFooterCommandsOpenScrollableHelpFromInputAndOtherScreens(t *testing.T) {
	for _, screen := range []string{"sessions", "about", "settings", "sync", "compose", "create"} {
		t.Run(screen, func(t *testing.T) {
			m := newModel(t)
			m.open(screen)
			m.width, m.height = 20, 8
			if screen == "sessions" {
				key(m, "/")
			} else if !strings.Contains(strings.Join(m.footerRows(), "\n"), "^P Commands") {
				t.Fatal("small terminal shows no available shortcut help")
			}
			m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
			key(m, "Keys")
			key(m, "enter")
			if m.screen != "help" {
				t.Fatal("command footer did not open shortcut help")
			}
			m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
			if m.help.offset == 0 {
				t.Fatal("input focus prevents scrolling shortcut help")
			}
			key(m, "esc")
			if m.screen != screen {
				t.Fatal("shortcut help did not restore its original screen")
			}
		})
	}
}

func TestFullFooterUsesCurrentPageAndDialog(t *testing.T) {
	m := newModel(t)
	m.open("settings")
	clickSetting(t, m, "full")
	if got := m.prefs().LoadInterface().ShortcutMode(); got != config.ShortcutBarFull {
		t.Fatal("Full mode was not saved", got)
	}
	m.Update(tea.WindowSizeMsg{Width: 90, Height: 30})
	for _, tc := range []struct {
		screen, want, absent string
	}{
		{"activity", "D Clean finished", "^D Clear skill"},
		{"compose", "^D Clear skill", "D Clean finished"},
		{"vibe", "p Copy prompt", "^D Clear skill"},
		{"stats", "pgup/pgdown Scroll", "D Clean finished"},
		{"settings", "enter/space Select", "p Copy prompt"},
	} {
		m.open(tc.screen)
		rows := m.footerRows()
		footer := strings.Join(rows, "\n")
		if !strings.Contains(footer, tc.want) || strings.Contains(footer, tc.absent) {
			t.Errorf("%s shows the wrong page shortcuts: %v", tc.screen, rows)
		}
		if m.footerTop() != m.height-len(rows) {
			t.Errorf("%s did not reserve its wrapped footer: %v", tc.screen, rows)
		}
	}
	m.open("activity")
	key(m, "/")
	if footer := strings.Join(m.footerRows(), "\n"); !strings.Contains(footer, "enter Results") || strings.Contains(footer, "D Clean finished") {
		t.Errorf("search shows inactive shortcuts: %s", footer)
	}
	key(m, "esc")
	key(m, "X")
	if footer := strings.Join(m.fullFooterBindings(), "\n"); !strings.Contains(footer, "enter Accept") || strings.Contains(footer, "D Clean finished") {
		t.Errorf("confirmation shows inactive shortcuts: %s", footer)
	}
}

func TestFullFooterPagesFitSmallTerminals(t *testing.T) {
	m := newModel(t)
	m.interfacePrefs.ShortcutBar = config.ShortcutBarFull
	m.open("activity")
	m.activityRows = []activityRow{{entry: history.Invocation{ID: 1, Name: "Visible task"}}}
	for _, language := range languages[1:] {
		m.language = language
		for _, size := range [][2]int{{20, 8}, {40, 18}} {
			m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			allRows, visible := m.fullFooterLayout()
			count := (len(allRows) + visible - 1) / visible
			if count <= 1 {
				t.Fatal("test requires a footer with more than one page")
			}
			for page := range count {
				rows := m.footerRows()
				for i, row := range rows {
					want := ""
					if offset := page*visible + i; offset < len(allRows) {
						want = allRows[offset]
					}
					if i == len(rows)-1 {
						want += footerPager
					}
					if row != want || ansi.StringWidth(row) > m.width-4 {
						t.Fatalf("%s at %v has an invalid footer row %q, want %q", language, size, row, want)
					}
				}
				view := m.View()
				assertFrame(t, view, m.width, m.height)
				if !strings.Contains(ansi.Strip(view), "Visible task") {
					t.Fatal("footer covered the selected task")
				}
				m.Update(tea.KeyMsg{Type: tea.KeyF6})
			}
			if m.footerPage != 0 {
				t.Fatal("footer did not return to its first page")
			}
		}
	}
}
