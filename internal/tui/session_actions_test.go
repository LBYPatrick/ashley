package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LBYPatrick/ashley/internal/sessions"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestSessionBulkActionsFromKeyboardMouseAndPalette(t *testing.T) {
	for _, route := range []string{"keyboard", "mouse", "palette"} {
		for _, action := range []string{"X", "cleanup"} {
			t.Run(route+"/"+action, func(t *testing.T) {
				m := newModel(t)
				alive, killed := true, 0
				manager := m.manager()
				manager.Run = func(args []string, _ string) (string, error) {
					switch args[0] {
					case "has-session":
						if alive && args[2] == "running" {
							return "", nil
						}
						return "", errors.New("not running")
					case "kill-session":
						alive = false
						killed++
					}
					return "", nil
				}
				for _, id := range []string{"running", "finished"} {
					s := sessions.Session{ID: id, TmuxSession: id, Skill: "coding", LogFile: filepath.Join(manager.Dir, id+".log")}
					if err := manager.Save(s); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(s.LogFile, []byte("Saved output"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				m.options.Sessions = &manager
				m.open("sessions")
				trigger := func() {
					switch route {
					case "keyboard":
						shortcut := action
						if action == "cleanup" {
							shortcut = "D"
						}
						key(m, shortcut)
					case "mouse":
						for _, control := range m.sessionActionControls() {
							if control.key == action {
								m.Update(tea.MouseMsg{X: control.x + 1, Y: control.y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
							}
						}
					case "palette":
						m.paletteOpen = true
						m.paletteQuery = "Kill all sessions"
						if action == "cleanup" {
							m.paletteQuery = "Clean finished sessions"
						}
						m.paletteCursor = 0
						key(m, "enter")
					}
				}
				trigger()
				if m.confirm == nil || m.confirm.key != action || m.confirm.proceed {
					t.Fatal("action did not open its confirmation with Cancel selected")
				}
				key(m, "enter")
				if rows, err := manager.All(); err != nil || len(rows) != 2 || killed != 0 {
					t.Fatal("cancel changed the sessions", rows, err)
				}
				trigger()
				key(m, "tab")
				key(m, "enter")
				if action == "cleanup" {
					if len(m.sessionRows) != 1 || m.sessionRows[0].ID != "running" || killed != 0 {
						t.Fatal("cleanup removed a running session or did not remove the finished record")
					}
				} else if len(m.sessionRows) != 0 || killed != 1 {
					t.Fatal("kill all did not stop the running session and remove the records")
				}
				for _, id := range []string{"running", "finished"} {
					if _, err := os.Stat(filepath.Join(manager.Dir, id+".log")); err != nil {
						t.Fatal("bulk action removed a saved log", err)
					}
				}
			})
		}
	}
}

func TestSessionBulkControlsFitAndStayInSessions(t *testing.T) {
	for _, width := range []int{20, 30, 50, 80, 120} {
		m := newModel(t)
		m.open("sessions")
		m.width, m.height = width, 20
		view := m.View()
		assertFrame(t, view, width, 20)
		for _, control := range m.sessionActionControls() {
			if control.x+control.w > m.layout().left.x+m.layout().left.w || !strings.Contains(ansi.Strip(view), control.label) {
				t.Fatalf("bulk action is hidden at width %d: %s", width, control.label)
			}
		}
		m.open("history")
		if len(m.sessionActionControls()) != 0 {
			t.Fatal("session controls appear in history")
		}
		key(m, "D")
		if m.confirm != nil {
			t.Fatal("session cleanup shortcut works in history")
		}
	}
}
