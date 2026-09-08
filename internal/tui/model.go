// Package tui implements Ashley's interactive terminal screens.
package tui

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/LBYPatrick/ashley/internal/config"
	"github.com/LBYPatrick/ashley/internal/history"
	"github.com/LBYPatrick/ashley/internal/sessions"
	"github.com/LBYPatrick/ashley/internal/skills"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// Options supplies application data and an optional custom repository.
type Options struct {
	Catalog            skills.Catalog
	Home, Root, Screen string
	Execute            func([]string) tea.Cmd
}
type completed struct{ err error }
type tick time.Time

var hub = []string{"Vibe", "Sessions", "History", "Generate", "Install", "Create", "Stats", "Settings"}
var modes = []string{"default", "dsp", "auto", "afk"}

// Model holds terminal UI state; IO operations are isolated in refresh/actions.
type Model struct {
	wizard                              *creatorWizard
	sessionAlive                        map[string]bool
	sessionLog                          string
	logOffset                           int
	paletteOpen                         bool
	paletteQuery                        string
	paletteCursor                       int
	maximized                           bool
	screenScroll                        int
	editor                              textarea.Model
	options                             Options
	screen                              string
	cursor, width, height, mode, offset int
	names                               []string
	filter, question                    textinput.Model
	focus                               string
	preview                             viewport.Model
	theme                               config.Theme
	agent, status, sortMode             string
	sessionRows                         []sessions.Session
	historyRows                         []history.Invocation
	stats                               history.Stats
	total                               int
	settingsRow, settingsColumn         int
	firstRun                            bool
}

// New loads saved appearance and the initial screen.
func New(options Options) (*Model, error) {
	if options.Home == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		options.Home = home
	}
	prefs := config.Store{Dir: filepath.Join(options.Home, ".ashley")}
	screen := options.Screen
	if screen == "" {
		screen = "hub"
	}
	m := &Model{options: options, screen: screen, width: 100, height: 30, theme: prefs.LoadTheme(), agent: prefs.LoadAgent(), sortMode: "time"}
	m.editor = textarea.New()
	m.filter = textinput.New()
	m.filter.Placeholder = "Search…"
	m.filter.CharLimit = 0
	m.question = textinput.New()
	m.question.Placeholder = "Enter a question, then Enter to run"
	m.question.CharLimit = 0
	m.preview = viewport.New(80, 20)
	if !prefs.ThemeConfigured() {
		m.screen = "settings"
		m.firstRun = true
		m.focusSettings()
	}
	m.refresh()
	if m.screen == "create" {
		m.startCreator()
	}
	return m, nil
}
func (m *Model) prefs() config.Store {
	return config.Store{Dir: filepath.Join(m.options.Home, ".ashley")}
}
func (m *Model) manager() sessions.Manager {
	return sessions.Manager{Dir: filepath.Join(m.options.Home, ".ashley", "sessions"), Run: sessions.Tmux}
}
func (m *Model) db() (*history.Store, error) {
	return history.Open(history.Path(m.options.Home, runtime.GOOS, os.Getenv("XDG_DATA_HOME")))
}
func (m *Model) refresh() {
	m.status = ""
	switch m.screen {
	case "vibe":
		names, err := m.options.Catalog.Names()
		if err != nil {
			m.status = err.Error()
			break
		}
		m.names = nil
		for _, name := range names {
			def, err := m.options.Catalog.Load(name)
			if err != nil {
				m.status = err.Error()
				continue
			}
			if strings.Contains(strings.ToLower(name+" "+def.Description), strings.ToLower(m.filter.Value())) {
				m.names = append(m.names, name)
			}
		}
	case "sessions":
		m.sessionAlive = nil
		rows, err := m.manager().All()
		if err != nil {
			m.status = err.Error()
		} else {
			m.sessionRows = sessions.Sort(rows, m.sortMode)
		}
	case "history", "stats":
		store, err := m.db()
		if err != nil {
			m.status = err.Error()
			break
		}
		defer store.Close()
		if m.screen == "history" {
			f := history.Filter{Search: m.filter.Value()}
			m.historyRows, err = store.Query(f, 50, m.offset)
			if err == nil {
				m.total, err = store.Count(f)
			}
		} else {
			m.stats, err = store.Stats(history.Filter{})
		}
		if err != nil {
			m.status = err.Error()
		}
	}
	m.cursor = max(0, min(m.cursor, m.count()-1))
	m.updatePreview()
}
func (m *Model) count() int {
	switch m.screen {
	case "hub":
		return len(hub)
	case "vibe":
		return len(m.names)
	case "sessions":
		return len(m.sessionRows)
	case "history":
		return len(m.historyRows)
	}
	return 0
}
func (m *Model) updatePreview() {
	if m.screen != "hub" && m.screen != "vibe" && m.screen != "sessions" && m.screen != "history" {
		return
	}
	m.logOffset = 0
	if m.screen == "sessions" {
		m.refreshSessionDetails()
	}
	l := m.layout()
	m.preview.Width = l.detail.w
	m.preview.Height = l.detail.h
	if m.screen == "sessions" {
		m.preview.Height = max(3, l.detail.h-7)
	}
	m.preview.SetContent(ansi.Wrap(m.detailText(), max(1, l.detail.w), ""))
	m.preview.GotoTop()
}

// Init starts periodic session refreshes.
func (m *Model) Init() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return tick(t) })
}
func (m *Model) execute(args ...string) tea.Cmd {
	if m.options.Execute != nil {
		return m.options.Execute(args)
	}
	executable, err := os.Executable()
	if err != nil {
		return func() tea.Msg { return completed{err} }
	}
	if m.options.Root != "" {
		args = append([]string{"--root", m.options.Root}, args...)
	}
	return tea.ExecProcess(exec.Command(executable, args...), func(err error) tea.Msg { return completed{err} })
}
func (m *Model) open(screen string) {
	m.screen = screen
	m.screenScroll = 0
	m.cursor = 0
	m.offset = 0
	m.focus = ""
	m.filter.SetValue("")
	m.filter.Blur()
	m.question.Blur()
	m.refresh()
	if screen == "settings" {
		m.focusSettings()
	}
	if screen == "create" {
		m.startCreator()
	}
}

// Update handles navigation, editable fields, and screen-specific actions.
func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.MouseMsg:
		return m, m.mouse(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.preview.Width = max(20, msg.Width*2/3-6)
		m.preview.Height = max(3, msg.Height-10)
		m.filter.Width = max(10, msg.Width-20)
		m.question.Width = max(10, msg.Width-20)
		m.editor.SetWidth(max(20, msg.Width-8))
		m.editor.SetHeight(max(3, msg.Height-10))
		m.updatePreview()
		if m.wizard != nil {
			m.wizard.input.SetWidth(max(20, msg.Width-8))
			m.wizard.input.SetHeight(max(3, msg.Height-17))
		}
	case completed:
		m.refresh()
		if msg.err != nil {
			m.status = msg.err.Error()
		} else {
			m.status = "Completed."
		}
	case tick:
		if m.screen == "sessions" {
			selected := ""
			if len(m.sessionRows) > 0 {
				selected = m.sessionRows[m.cursor].ID
			}
			detailOffset, logOffset := m.preview.YOffset, m.logOffset
			m.refresh()
			for index, session := range m.sessionRows {
				if session.ID == selected {
					m.cursor = index
					m.updatePreview()
					m.preview.SetYOffset(detailOffset)
					m.logOffset = logOffset
					break
				}
			}
		}
		return m, m.Init()
	case tea.KeyMsg:
		key := msg.String()
		if m.paletteOpen {
			return m, m.paletteKey(msg)
		}
		if key == "ctrl+p" {
			m.paletteOpen = true
			m.paletteQuery = ""
			m.paletteCursor = 0
			return m, nil
		}
		if key == "esc" && m.maximized {
			m.maximized = false
			return m, nil
		}
		if m.screen == "create" && key != "ctrl+c" {
			cmd := m.createKey(msg)
			m.keepCreatorVisible()
			return m, cmd
		}
		if m.screen == "create-preview" && key == "esc" {
			m.screen = "create"
			return m, nil
		}
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		if m.focus != "" {
			if key == "tab" {
				if m.focus == "filter" && m.screen == "vibe" {
					m.focus = "question"
					m.filter.Blur()
					return m, m.question.Focus()
				}
				m.focus = ""
				m.filter.Blur()
				m.question.Blur()
				return m, nil
			}
			if key == "esc" {
				m.focus = ""
				m.filter.Blur()
				m.question.Blur()
				return m, nil
			}
			if key == "enter" {
				if m.focus == "question" && len(m.names) > 0 {
					args := []string{"run"}
					switch modes[m.mode] {
					case "default":
						args = append(args, "--normal")
					case "dsp":
						args = append(args, "--dangerously-skip-permissions")
					case "auto":
						args = append(args, "--auto")
					case "afk":
						args = append(args, "--away-from-keyboard")
					}
					args = append(args, m.names[m.cursor], "--", m.question.Value())
					return m, m.execute(args...)
				}
				m.focus = ""
				m.filter.Blur()
				return m, nil
			}
			var cmd tea.Cmd
			if m.focus == "filter" {
				m.filter, cmd = m.filter.Update(msg)
				m.offset = 0
				m.cursor = 0
				m.refresh()
			} else {
				m.question, cmd = m.question.Update(msg)
			}
			return m, cmd
		}
		if m.screen == "settings" {
			return m, m.settingsKey(key)
		}
		if key == "esc" {
			if m.screen == "log" {
				m.open("sessions")
				return m, nil
			}
			if m.screen == "history" && m.options.Screen == "history" && m.filter.Value() != "" {
				m.filter.SetValue("")
				m.offset = 0
				m.refresh()
				return m, nil
			}
			if m.screen == "hub" {
				return m, tea.Quit
			}
			m.open("hub")
			return m, nil
		}
		if key == "q" && (m.screen == "hub" || (m.options.Screen == m.screen && (m.screen == "history" || m.screen == "sessions"))) {
			return m, tea.Quit
		}
		switch key {
		case "up", "k":
			if key == "k" && m.screen == "sessions" {
				n, err := m.manager().CleanDead()
				m.refresh()
				m.status = fmt.Sprintf("Cleaned %d sessions: %v", n, err)
			} else {
				m.cursor = max(0, m.cursor-1)
				m.updatePreview()
			}
		case "down", "j":
			m.cursor = min(max(0, m.count()-1), m.cursor+1)
			m.updatePreview()
		case "/":
			if m.screen == "vibe" || m.screen == "history" {
				m.focus = "filter"
				return m, m.filter.Focus()
			}
		case "tab":
			if m.screen == "vibe" {
				m.focus = "question"
				return m, m.question.Focus()
			}
		case "m":
			if m.screen == "vibe" {
				m.mode = (m.mode + 1) % len(modes)
			}
		case "r":
			m.refresh()
		case "enter":
			return m, m.activate()
		case "p":
			if m.screen == "vibe" {
				return m, m.copyPrompt()
			}
			if m.screen == "history" {
				m.offset = max(0, m.offset-50)
				m.refresh()
			}
		case "n":
			if m.screen == "history" && m.offset+50 < m.total {
				m.offset += 50
				m.refresh()
			}
		case "s":
			if m.screen == "sessions" {
				if m.sortMode == "time" {
					m.sortMode = "skill"
				} else {
					m.sortMode = "time"
				}
				m.refresh()
			}
		case "c", "l", "K", "X", "d":
			return m, m.recordAction(key)
		case "pgup", "pgdown":
			if m.screen == "stats" {
				delta := max(1, m.height-8)
				if key == "pgup" {
					delta = -delta
				}
				m.screenScroll = max(0, m.screenScroll+delta)
				return m, nil
			}
			var cmd tea.Cmd
			m.preview, cmd = m.preview.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}
func (m *Model) activate() tea.Cmd {
	switch m.screen {
	case "hub":
		key := strings.ToLower(hub[m.cursor])
		switch key {
		case "generate":
			return m.execute("generate")
		case "install":
			return m.execute("install")
		default:
			m.open(key)
		}
	case "vibe":
		m.focus = "question"
		return m.question.Focus()
	case "sessions":
		if len(m.sessionRows) > 0 {
			return m.execute("attach", m.sessionRows[m.cursor].ID)
		}
	}
	return nil
}
func (m *Model) settingsKey(key string) tea.Cmd {
	rows := settingsRows()
	switch key {
	case "up":
		m.settingsRow = max(0, m.settingsRow-1)
	case "down", "tab":
		m.settingsRow = min(len(rows)-1, m.settingsRow+1)
	case "left":
		m.settingsColumn = max(0, m.settingsColumn-1)
	case "right":
		m.settingsColumn = min(len(rows[m.settingsRow])-1, m.settingsColumn+1)
	case "enter", " ":
		value := rows[m.settingsRow][min(m.settingsColumn, len(rows[m.settingsRow])-1)]
		switch m.settingsRow {
		case 0:
			m.agent = value
		case 1:
			m.theme.Mode = value
		case 2, 3:
			m.theme.Preset = value
		case 4:
			return m.finishSettings()
		}
		if err := m.prefs().SaveAgent(m.agent); err != nil {
			m.status = err.Error()
		}
		if err := m.prefs().SaveTheme(m.theme); err != nil {
			m.status = err.Error()
		}
	case "esc":
		return m.finishSettings()
	}
	m.settingsColumn = min(m.settingsColumn, len(rows[m.settingsRow])-1)
	m.keepSettingVisible()
	return nil
}
func (m *Model) finishSettings() tea.Cmd {
	if err := m.prefs().SaveTheme(m.theme); err != nil {
		m.status = err.Error()
		return nil
	}
	if err := m.prefs().SaveAgent(m.agent); err != nil {
		m.status = err.Error()
		return nil
	}
	screen := "hub"
	if m.firstRun && m.options.Screen != "" {
		screen = m.options.Screen
	}
	m.firstRun = false
	m.open(screen)
	return nil
}
func (m *Model) recordAction(key string) tea.Cmd {
	if m.screen == "sessions" {
		if key == "X" {
			_, err := m.manager().KillAll()
			m.refresh()
			if err != nil {
				m.status = err.Error()
			}
			return nil
		}
		if len(m.sessionRows) == 0 {
			return nil
		}
		s := m.sessionRows[m.cursor]
		switch key {
		case "c":
			return m.copyText(s.ID)
		case "l":
			text, err := sessions.ReadLog(s, 0)
			if err != nil {
				m.status = err.Error()
			} else {
				m.screen = "log"
				m.preview.SetContent(text)
			}
		case "K":
			err := m.manager().Kill(s)
			m.refresh()
			if err != nil {
				m.status = err.Error()
			}
		case "d":
			if m.manager().Alive(s) {
				m.status = "Kill the running session before deleting its log."
			} else {
				err := m.manager().Remove(s, true)
				m.refresh()
				if err != nil {
					m.status = err.Error()
				}
			}
		}
	} else if m.screen == "history" && key == "d" && len(m.historyRows) > 0 {
		store, err := m.db()
		if err == nil {
			_, err = store.Delete(m.historyRows[m.cursor].ID)
			store.Close()
		}
		m.refresh()
		if err != nil {
			m.status = err.Error()
		}
	}
	return nil
}
func (m *Model) copyPrompt() tea.Cmd {
	if len(m.names) == 0 {
		return nil
	}
	doc, err := m.options.Catalog.Document(m.names[m.cursor])
	if err != nil {
		m.status = err.Error()
		return nil
	}
	text, err := skills.Prompt(doc, m.question.Value(), nil)
	if err != nil {
		m.status = err.Error()
		return nil
	}
	return m.copyText(text)
}
func (m *Model) copyText(text string) tea.Cmd {
	return func() tea.Msg {
		var args []string
		if runtime.GOOS == "darwin" {
			args = []string{"pbcopy"}
		} else if _, err := exec.LookPath("wl-copy"); err == nil {
			args = []string{"wl-copy"}
		} else {
			args = []string{"xclip", "-selection", "clipboard"}
		}
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdin = strings.NewReader(text)
		return completed{cmd.Run()}
	}
}

// Run opens the terminal application and restores terminal state on exit.
func Run(options Options, output io.Writer) error {
	// Textual retains grayscale backgrounds under NO_COLOR; keep selection visible.
	previous := lipgloss.ColorProfile()
	if _, set := os.LookupEnv("NO_COLOR"); set {
		lipgloss.SetColorProfile(termenv.TrueColor)
		defer lipgloss.SetColorProfile(previous)
	}
	model, err := New(options)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithOutput(output)).Run()
	return err
}
