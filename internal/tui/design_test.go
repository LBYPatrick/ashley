package tui

import (
	"strings"
	"testing"

	"github.com/LBYPatrick/ashley/internal/history"
	"github.com/LBYPatrick/ashley/internal/sessions"
	"github.com/LBYPatrick/ashley/internal/skills"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestEveryCreatorStageAndViewerFitsTerminal(t *testing.T) {
	trueColor(t)
	for _, size := range [][2]int{{50, 20}, {100, 32}, {160, 45}} {
		m := newModel(t)
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m.open("create")
		w := m.wizard
		w.basics = [4]string{"design-demo", "A careful implementation", "", "Preserve the user's intent."}
		w.input.SetValue(w.basics[0])
		w.steps = []skills.Step{{Name: "Inspect", Instructions: []string{"Read relevant code and reproduce the issue."}, Validation: "The cause is understood."}}
		w.workflowName = "Implement and verify"
		for stage := 0; stage < 4; stage++ {
			if !m.wizardPreview() {
				t.Fatal(m.status)
			}
			w.stage = stage
			w.field = 0
			w.loadField()
			m.sizeCreatorEditor()
			view := m.View()
			assertFrame(t, view, size[0], size[1])
			if !strings.Contains(ansi.Strip(view), "Step ") {
				t.Fatal("creator has no progress indicator")
			}
			if size[0] == 100 {
				exportRegressionView(t, []string{"creator-basics", "creator-files", "creator-workflow", "creator-preview"}[stage], view)
			}
		}
		m.wizard = nil
		m.sizeCreatorEditor()
		assertFrame(t, m.View(), size[0], size[1])
		if size[0] == 100 {
			exportRegressionView(t, "creator-json", m.View())
		}
		for _, screen := range []string{"log", "help", "create-preview"} {
			m.screen = screen
			m.logContent = "Readable output\n\n" + strings.Repeat("Line of output\n", 60) + "END"
			m.sizeLogPreview()
			m.preview.GotoBottom()
			view := m.View()
			assertFrame(t, view, size[0], size[1])
			if !strings.Contains(ansi.Strip(view), "END") {
				t.Fatal("viewer clips its final line", screen)
			}
		}
	}
}
func assertFrame(t *testing.T, view string, w, h int) {
	t.Helper()
	rows := strings.Split(view, "\n")
	if len(rows) != h {
		t.Fatal("wrong frame height", len(rows), h)
	}
	for _, row := range rows {
		if ansi.StringWidth(row) != w {
			t.Fatal("frame escaped terminal", ansi.StringWidth(row), w)
		}
	}
	if len(view) > w*h*20 {
		t.Fatal("excessive ANSI output")
	}
}
func TestPopulatedBrowsersPreserveAllDetails(t *testing.T) {
	trueColor(t)
	m := newModel(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m.open("history")
	m.historyRows = []history.Invocation{{ID: 183, Skill: "feat", Question: "Polish every terminal page", CWD: "/workspace/ashley", AgentType: "codex", Permission: "auto", Detached: true, SessionID: "demo-session"}}
	m.total = 183
	m.updatePreview()
	view := m.View()
	for _, field := range []string{"History (183)", "Page 1/4", "Invocation #183", "Question:", "Directory:", "Permission:", "Detached:", "Database:"} {
		if !strings.Contains(ansi.Strip(view), field) {
			t.Fatal(field)
		}
	}
	exportRegressionView(t, "history-populated", view)
	m.open("sessions")
	m.sessionRows = []sessions.Session{{ID: "demo-session", Skill: "feat", Question: "Polish every terminal page", CWD: "/workspace/ashley", Agent: "codex", PermissionMode: "auto", LogFile: "/tmp/demo.log", TmuxSession: "demo"}}
	m.sessionAlive = map[string]bool{"demo-session": true}
	m.sessionLog = "Inspecting the application…\nFound the issue.\nImplementation complete.\nAll checks passed."
	exportRegressionView(t, "sessions-populated", m.View())
	m.open("stats")
	m.stats = history.Stats{Total: 183, TopSkills: []history.Usage{{Name: "feat", Count: 80}, {Name: "coding", Count: 60}, {Name: "debug", Count: 25}, {Name: "commit", Count: 18}}, ByAgent: []history.Usage{{Name: "codex", Count: 140}, {Name: "claude", Count: 43}}}
	exportRegressionView(t, "analytics", m.View())
	// Repeated wheel input at an edge must not create an invisible scroll debt.
	for range 200 {
		m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	}
	if m.screenScroll != m.statsMaxScroll() {
		t.Fatal("unbounded analytics scroll")
	}
}

func TestBrowserLayoutUsesWideTerminalAndResizes(t *testing.T) {
	for _, screen := range []string{"vibe", "sessions", "history"} {
		m := newModel(t)
		m.open(screen)
		for _, width := range []int{80, 190, 260, 100} {
			m.Update(tea.WindowSizeMsg{Width: width, Height: 40})
			l := m.layout()
			if l.left.x != 2 || l.right.x+l.right.w != width-2 {
				t.Fatalf("%s at %d leaves unused margins: %+v", screen, width, l)
			}
			if screen == "vibe" && (l.input.w != min(132, width-4) || l.mode.x != l.input.x || l.mode.w != l.input.w) {
				t.Fatalf("prompt and mode controls must share bounded edges: %+v", l)
			}
			if m.preview.Width != l.detail.w {
				t.Fatalf("%s viewport did not resize: %d != %d", screen, m.preview.Width, l.detail.w)
			}
			assertFrame(t, m.View(), width, 40)
			if width == 190 {
				exportRegressionView(t, screen+"-wide", m.View())
			}
		}
	}
}
