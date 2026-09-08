package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ashley "github.com/LBYPatrick/ashley"
	"github.com/LBYPatrick/ashley/internal/history"
	"github.com/LBYPatrick/ashley/internal/sessions"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestOriginalPythonScreenLayouts(t *testing.T) {
	if os.Getenv("ASHLEY_UI_EXPORT") != "" {
		previous := lipgloss.ColorProfile()
		lipgloss.SetColorProfile(termenv.TrueColor)
		t.Cleanup(func() { lipgloss.SetColorProfile(previous) })
	}
	for _, screen := range []string{"hub", "vibe", "sessions", "history", "stats", "settings", "create"} {
		t.Run(screen, func(t *testing.T) {
			m := newModel(t)
			m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
			m.open(screen)
			rendered := m.View()
			if directory := os.Getenv("ASHLEY_UI_EXPORT"); directory != "" {
				os.MkdirAll(directory, 0700)
				os.WriteFile(filepath.Join(directory, screen+".txt"), []byte(ansi.Strip(rendered)), 0600)
				os.WriteFile(filepath.Join(directory, screen+".ansi"), []byte(rendered), 0600)
			}
			rows := strings.Split(ansi.Strip(rendered), "\n")
			if !strings.Contains(rows[0], "Ashley v"+ashley.Version()) {
				t.Fatal("missing original header", rows[0])
			}
			for index := range rows {
				rows[index] = strings.TrimRight(rows[index], " ")
			}
			actual := strings.Join(rows[1:], "\n") + "\n"
			expected, err := os.ReadFile(filepath.Join("..", "..", "testdata", "ui", "python", screen+".txt"))
			if err != nil {
				t.Fatal(err)
			}
			if actual != string(expected) {
				t.Fatalf("%s differs from original Python layout\n%s", screen, actual)
			}
		})
	}
}
func TestScreensStayInsideTerminalBounds(t *testing.T) {
	for _, size := range [][2]int{{50, 20}, {80, 24}, {100, 30}, {140, 50}} {
		for _, screen := range []string{"hub", "vibe", "sessions", "history", "stats", "settings", "create"} {
			m := newModel(t)
			m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			m.open(screen)
			rows := strings.Split(m.View(), "\n")
			if len(rows) != size[1] {
				t.Fatalf("%s height %d, want %d", screen, len(rows), size[1])
			}
			for index, row := range rows {
				if ansi.StringWidth(row) != size[0] {
					t.Fatalf("%s row %d width %d, want %d", screen, index, ansi.StringWidth(row), size[0])
				}
			}
		}
	}
}

func TestOriginalInformationPanelsAndIndependentLogScroll(t *testing.T) {
	m := newModel(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 45})
	m.open("vibe")
	for index, name := range m.names {
		if name == "feat" {
			m.cursor = index
		}
	}
	m.updatePreview()
	for _, field := range []string{"a-feat", "Overview", "Extends", "refactor", "Components", "Resources", "Steps", "Workflow", "Understand the Spec"} {
		if !strings.Contains(ansi.Strip(m.View()), field) {
			t.Fatal("missing skill information", field)
		}
	}
	log := filepath.Join(t.TempDir(), "run.log")
	os.WriteFile(log, []byte("first log line\n"+strings.Repeat("more output\n", 35)+"last log line"), 0600)
	m.open("sessions")
	m.options.Screen = "sessions"
	m.sessionRows = []sessions.Session{{ID: "uitest123", Skill: "feat", Question: "Investigate missing details", StartedAt: "2026-09-08T00:00:00Z", CWD: "/workspace/example", PermissionMode: "auto", Agent: "codex", TmuxSession: "ashley-ui-test-nonexistent", LogFile: log}}
	m.updatePreview()
	for _, field := range []string{"Session uitest123", "Status:", "Question:", "Started:", "Elapsed:", "Directory:", "Permission:", "tmux:", "Log:", "Log (last 50 lines)", "first log line"} {
		if !strings.Contains(ansi.Strip(m.View()), field) {
			t.Fatal("missing session information", field)
		}
	}
	offset := m.preview.YOffset
	m.Update(tea.MouseMsg{X: 80, Y: 30, Button: tea.MouseButtonWheelDown})
	if m.logOffset == 0 || m.preview.YOffset != offset {
		t.Fatal("log scroll moved the detail pane", m.logOffset, m.preview.YOffset)
	}
	m.open("history")
	m.historyRows = []history.Invocation{{ID: 42, Skill: "feat", Question: "Why is this missing?", CWD: "/workspace/example", AgentType: "codex", Permission: "auto", Detached: true, SessionID: "uitest123"}}
	m.updatePreview()
	for _, field := range []string{"Invocation #42", "Time:", "Skill:", "Agent:", "Question:", "Directory:", "Permission:", "Detached:", "Database:"} {
		if !strings.Contains(ansi.Strip(m.View()), field) {
			t.Fatal("missing history information", field)
		}
	}
}

func TestPaletteNavigationAndMonochromeSelection(t *testing.T) {
	m := newModel(t)
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	if !m.paletteOpen || !strings.Contains(m.View(), "Search for commands") {
		t.Fatal("palette unavailable")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("theme")})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != "settings" || m.paletteOpen {
		t.Fatal("theme command did not open settings")
	}
	t.Setenv("NO_COLOR", "1")
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(previous)
	m.open("hub")
	rendered := m.View()
	if !strings.Contains(rendered, "\x1b[") {
		t.Fatal("NO_COLOR removed all panel/selection styling")
	}
	if terminalColor("#4A9EFF") != "#939393" {
		t.Fatal("monochrome differs from original Textual luminance")
	}
}

func TestSessionRefreshPreservesSelectionAndScroll(t *testing.T) {
	m := newModel(t)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	manager := m.manager()
	session := sessions.Session{ID: "older-ui-session", Skill: "feat", TmuxSession: "ashley-ui-missing", StartedAt: "2026-01-01T00:00:00Z", LogFile: filepath.Join(manager.Dir, "old.log")}
	if err := manager.Save(session); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(session.LogFile, []byte(strings.Repeat("line\n", 30)), 0600)
	m.open("sessions")
	m.preview.SetYOffset(2)
	m.logOffset = 4
	newer := session
	newer.ID = "newer-ui-session"
	newer.StartedAt = "2026-02-01T00:00:00Z"
	if err := manager.Save(newer); err != nil {
		t.Fatal(err)
	}
	m.Update(tick(time.Now()))
	if m.sessionRows[m.cursor].ID != session.ID || m.logOffset != 4 || m.preview.YOffset != 2 {
		t.Fatal("refresh moved the selected session or scroll position", m.cursor, m.logOffset, m.preview.YOffset)
	}
}
