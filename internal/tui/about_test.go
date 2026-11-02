package tui

import (
	"strings"
	"testing"

	"github.com/LBYPatrick/ashley"
	"github.com/LBYPatrick/ashley/internal/history"
	"github.com/LBYPatrick/ashley/internal/sessions"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestAboutUsesEmbeddedVersion(t *testing.T) {
	version, err := ashley.Assets.ReadFile("VERSION")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Version: " + strings.TrimSpace(string(version)), "https://github.com/LBYPatrick/ashley", "License: MIT", "Copyright (c) 2025 Ashley Contributors"} {
		if !strings.Contains(aboutText(), want) {
			t.Fatalf("About omits %q", want)
		}
	}
}

func TestAboutHomeAndPaletteAccess(t *testing.T) {
	for _, route := range []string{"home keyboard", "home mouse", "palette about", "palette version"} {
		t.Run(route, func(t *testing.T) {
			m := newModel(t)
			m.width, m.height = 100, 20
			if strings.HasPrefix(route, "home") {
				m.cursor = len(hub) - 1
				controls := m.homeControls()
				control := controls[m.cursor]
				if control.label != "About Ashley" || control.y >= m.height-2 || !strings.Contains(ansi.Strip(m.View()), "About Ashley") {
					t.Fatal("Home does not show the selected About control", control)
				}
				if route == "home keyboard" {
					key(m, "enter")
				} else {
					m.Update(tea.MouseMsg{X: control.x, Y: control.y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
				}
			} else {
				m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
				key(m, strings.TrimPrefix(route, "palette "))
				key(m, "enter")
			}
			if m.screen != "about" || !strings.Contains(ansi.Strip(m.View()), "Version: "+ashley.Version()) {
				t.Fatal("About did not open with the embedded version", m.screen)
			}
			exportRegressionView(t, "about", m.View())
			key(m, "esc")
			if m.screen != "hub" {
				t.Fatal("About did not return Home")
			}
		})
	}
}

func TestAboutRetainsBrowserAndReadingState(t *testing.T) {
	for _, screen := range []string{"sessions", "history", "log", "help"} {
		t.Run(screen, func(t *testing.T) {
			m := newModel(t)
			m.screen, m.cursor, m.offset = screen, 1, 50
			m.filter.SetValue("billing")
			m.directoryOnly, m.compactDetail, m.maximized = true, true, true
			m.focus = "filter"
			m.screenScroll, m.logOffset = 4, 3
			m.sessionRows = []sessions.Session{{ID: "first"}, {ID: "selected"}}
			m.historyRows = []history.Invocation{{ID: 1}, {ID: 2}}
			m.logContent = strings.Repeat("Saved log or help content\n", 50)
			m.preview.SetContent(m.logContent)
			m.preview.SetYOffset(7)
			m.helpReturn = "log"
			m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
			key(m, "version")
			key(m, "enter")
			if m.screen != "about" {
				t.Fatal("palette did not open About")
			}
			m.Update(tea.KeyMsg{Type: tea.KeyEnd})
			key(m, "esc")
			if m.screen != screen || m.cursor != 1 || m.offset != 50 || m.filter.Value() != "billing" || !m.directoryOnly || m.focus != "filter" {
				t.Fatal("About changed browser state")
			}
			if !m.compactDetail || !m.maximized || m.screenScroll != 4 || m.logOffset != 3 || m.helpReturn != "log" || m.logContent != strings.Repeat("Saved log or help content\n", 50) {
				t.Fatal("About changed reading state")
			}
			if (screen == "log" || screen == "help") && m.preview.YOffset != 7 {
				t.Fatal("About changed the log or help scroll position", m.preview.YOffset)
			}
		})
	}
}

func TestAboutSmallScreenScrolling(t *testing.T) {
	m := newModel(t)
	m.width, m.height = 20, 8
	m.openAbout()
	assertFrame(t, m.View(), 20, 8)
	if m.aboutMaxScroll() <= 0 {
		t.Fatal("small About view must scroll")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.screenScroll != 1 {
		t.Fatal("Down did not scroll")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.screenScroll != 0 {
		t.Fatal("Up did not scroll")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if m.screenScroll != m.readingRect().h {
		t.Fatal("PageDown did not move by the reading height")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.screenScroll != 0 {
		t.Fatal("PageUp did not move by the reading height")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if m.screenScroll != m.aboutMaxScroll() || !strings.Contains(ansi.Strip(m.View()), "Contributors") {
		t.Fatal("End does not show the final About text", ansi.Strip(m.View()))
	}
	m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	if m.screenScroll != max(0, m.aboutMaxScroll()-3) {
		t.Fatal("mouse wheel did not scroll up")
	}
	m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	if m.screenScroll != m.aboutMaxScroll() {
		t.Fatal("mouse wheel did not scroll down")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyHome})
	if m.screenScroll != 0 {
		t.Fatal("Home did not return to the first line")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if m.screenScroll != 0 {
		t.Fatal("resize did not bound the scroll offset")
	}
	assertFrame(t, m.View(), 120, 40)
}
