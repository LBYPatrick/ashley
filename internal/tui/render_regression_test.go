package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/LBYPatrick/ashley/internal/history"
	"github.com/LBYPatrick/ashley/internal/sessions"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/cellbuf"
	"github.com/muesli/termenv"
)

func trueColor(t *testing.T) {
	t.Helper()
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(old) })
}
func TestColoredScreensHaveBoundedOutput(t *testing.T) {
	trueColor(t)
	for _, size := range [][2]int{{80, 24}, {100, 30}, {160, 50}} {
		m := newModel(t)
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for _, screen := range []string{"hub", "settings", "history", "sessions", "stats", "vibe"} {
			m.open(screen)
			start := time.Now()
			view := m.View()
			if len(view) > size[0]*size[1]*20 {
				t.Fatalf("%s generated %d bytes for %dx%d", screen, len(view), size[0], size[1])
			}
			if elapsed := time.Since(start); elapsed > time.Second {
				t.Fatalf("%s stalled for %s", screen, elapsed)
			}
			for _, row := range strings.Split(view, "\n") {
				if ansi.StringWidth(row) != size[0] {
					t.Fatalf("%s row escaped panel: %q", screen, row)
				}
			}
		}
	}
}
func TestHistorySelectionDoesNotSpillIntoDetails(t *testing.T) {
	trueColor(t)
	m := newModel(t)
	m.open("history")
	m.historyRows = []history.Invocation{{ID: 42, Skill: "coding", Question: "test", CWD: "/workspace"}}
	m.updatePreview()
	view := m.View()
	exportRegressionView(t, "history-populated", view)
	if !strings.Contains(ansi.Strip(view), "Question:   test") {
		t.Fatal("detail missing")
	}
	buffer := cellbuf.NewBuffer(m.width, m.height)
	cellbuf.SetContent(buffer, view)
	l := m.layout()
	selected := buffer.Cell(l.list.x, l.list.y)
	detail := buffer.Cell(l.detail.x, l.list.y)
	if reflect.DeepEqual(selected.Style.Bg, detail.Style.Bg) {
		t.Fatal("selection background leaked into right panel")
	}
}
func TestGradientMatchesOriginalTextual(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	os.Unsetenv("NO_COLOR")
	for accent, want := range map[string][]string{
		"#4A9EFF": {"#00489C", "#0071CC", "#4A9EFF", "#85CDFF", "#BCFEFF"},
		"#39C5CF": {"#006772", "#00959F", "#39C5CF", "#75F6FF", "#ACFFFF"},
		"#F85149": {"#880000", "#BF1321", "#F75149", "#FF8373", "#FFB5A1"},
	} {
		if got := accentGradient(accent, 5); !reflect.DeepEqual(got, want) {
			t.Fatal(accent, got, want)
		}
	}
	trueColor(t)
	m := newModel(t)
	m.open("stats")
	m.stats = history.Stats{Total: 35, TopSkills: []history.Usage{{Name: "feat", Count: 16}, {Name: "coding", Count: 11}, {Name: "raw", Count: 8}}}
	view := m.View()
	exportRegressionView(t, "stats-populated", view)
	// The middle bar rounds 16.5 to 16, matching Python, with its own tint.
	if !strings.Contains(view, "38;2;0;72;156") || !strings.Contains(view, "38;2;188;254;255") {
		t.Fatal("missing gradient colors", view)
	}
	if !strings.Contains(ansi.Strip(view), "coding       "+strings.Repeat("█", 16)+" 11") {
		t.Fatal("incorrect bar width", ansi.Strip(view))
	}
}
func TestSessionLogScrollUsesWrappedViewport(t *testing.T) {
	m := newModel(t)
	m.open("sessions")
	// A single raw line occupies many visual rows: all must remain reachable.
	m.sessionLog = strings.Repeat("wrapped output ", 150) + "TAIL"
	_, log, body := m.sessionPanels()
	expected := len(strings.Split(ansi.Wrap(m.sessionLog, body.w, ""), "\n")) - body.h
	for range 100 {
		m.Update(tea.MouseMsg{X: log.x + 1, Y: log.y + 1, Button: tea.MouseButtonWheelDown})
	}
	if m.logOffset != expected {
		t.Fatal(m.logOffset, expected)
	}
	if !strings.Contains(ansi.Strip(m.View()), "TAIL") {
		t.Fatal("last wrapped log line unreachable")
	}
	for range 100 {
		m.Update(tea.MouseMsg{X: log.x + 1, Y: log.y + 1, Button: tea.MouseButtonWheelUp})
	}
	if m.logOffset != 0 {
		t.Fatal(m.logOffset)
	}
}

func exportRegressionView(t *testing.T, name, view string) {
	t.Helper()
	if dir := os.Getenv("ASHLEY_UI_EXPORT"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".ansi"), []byte(view), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFullLogWrapsAndResizesWithoutLosingTail(t *testing.T) {
	m := newModel(t)
	m.open("sessions")
	path := filepath.Join(t.TempDir(), "log")
	os.WriteFile(path, []byte("progress\r\x1b[2K"+strings.Repeat("long output ", 100)+"TAIL"), 0600)
	m.sessionRows = []sessions.Session{{ID: "log-view", LogFile: path}}
	key(m, "l")
	if m.screen != "log" || strings.ContainsAny(m.logContent, "\r\x1b") {
		t.Fatal("raw terminal instructions in full log")
	}
	for _, width := range []int{100, 60, 140} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 24})
		m.preview.GotoBottom()
		if !strings.Contains(ansi.Strip(m.View()), "TAIL") {
			t.Fatal("full log tail clipped at width", width)
		}
	}
}
