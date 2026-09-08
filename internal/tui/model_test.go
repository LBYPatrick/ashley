package tui

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	ashley "github.com/LBYPatrick/ashley"
	"github.com/LBYPatrick/ashley/internal/config"
	"github.com/LBYPatrick/ashley/internal/history"
	"github.com/LBYPatrick/ashley/internal/sessions"
	"github.com/LBYPatrick/ashley/internal/skills"
	tea "github.com/charmbracelet/bubbletea"
)

func newModel(t *testing.T) *Model {
	t.Helper()
	home := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	prefs := config.Store{Dir: filepath.Join(home, ".ashley")}
	if err := prefs.SaveTheme(config.Theme{Mode: "dark", Preset: "blue"}); err != nil {
		t.Fatal(err)
	}
	m, err := New(Options{Catalog: skills.Catalog{Source: ashley.Assets}, Home: home})
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func key(m *Model, key string) {
	var msg tea.KeyMsg
	switch key {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		msg = tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		msg = tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		msg = tea.KeyMsg{Type: tea.KeyRight}
	case "tab":
		msg = tea.KeyMsg{Type: tea.KeyTab}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	m.Update(msg)
}
func TestHubSkillBrowserAndRunModes(t *testing.T) {
	m := newModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if !strings.Contains(m.View(), "Vibe") {
		t.Fatal(m.View())
	}
	key(m, "enter")
	if m.screen != "vibe" || len(m.names) != 14 {
		t.Fatal(m.screen, m.names)
	}
	key(m, "down")
	key(m, "up")
	if m.cursor != 0 {
		t.Fatal(m.cursor)
	}
	for i := 0; i < 4; i++ {
		if !strings.Contains(m.View(), []string{"Normal", "DSP", "AUTO", "AFK"}[i]) {
			t.Fatal(m.View())
		}
		key(m, "m")
	}
	key(m, "/")
	key(m, "find and fix bugs")
	if len(m.names) != 1 || m.names[0] != "debug" {
		t.Fatal(m.names)
	}
	key(m, "enter")
	var args []string
	m.options.Execute = func(argv []string) tea.Cmd { args = argv; return nil }
	key(m, "tab")
	key(m, "hello --literal")
	key(m, "enter")
	if !reflect.DeepEqual(args, []string{"run", "--normal", "debug", "--", "hello --literal"}) {
		t.Fatal(args)
	}
	key(m, "esc")
	key(m, "esc")
	if m.screen != "hub" {
		t.Fatal(m.screen)
	}
	m.cursor = 3
	stubSyncAgents(t, m)
	m.options.Background = func(_ context.Context, argv []string) (string, error) {
		args = argv
		return "generated/a-feat/SKILL.md", nil
	}
	cmd := m.activate()
	if m.screen != "sync" || cmd == nil {
		t.Fatal("Sync did not open its result page")
	}
	m.Update(cmd())
	if len(args) < 1 || args[0] != "install" || m.job.busy {
		t.Fatal(args)
	}
	key(m, "esc")
	m.cursor = 3
	cmd = m.activate()
	if cmd == nil {
		t.Fatal("Sync did not restart")
	}
	m.Update(cmd())
	if m.screen != "sync" {
		t.Fatal("Install did not open")
	}
	key(m, "i")
	if !reflect.DeepEqual(args, []string{"install", "--agent", m.agent}) {
		t.Fatal(args)
	}

}
func TestSettingsPersistAllChoices(t *testing.T) {
	m := newModel(t)
	m.open("settings")
	key(m, "down")
	key(m, "enter")
	if m.agent != "codex" {
		t.Fatal(m.agent)
	}
	for range 4 {
		key(m, "tab")
	}
	key(m, "right")
	key(m, "enter")
	if m.theme.Mode != "light" {
		t.Fatal(m.theme)
	}
	key(m, "down")
	key(m, "right")
	key(m, "enter")
	if m.theme.Preset != "purple" {
		t.Fatal(m.theme)
	}
	key(m, "esc")
	if m.screen != "hub" {
		t.Fatal(m.screen)
	}
	if m.prefs().LoadAgent() != "codex" || m.prefs().LoadTheme() != m.theme {
		t.Fatal("preferences not saved")
	}
	os.Remove(filepath.Join(m.options.Home, ".ashley", "theme.json"))
	first, err := New(Options{Home: m.options.Home, Catalog: m.options.Catalog, Screen: "vibe"})
	if err != nil {
		t.Fatal(err)
	}
	if !first.firstRun || first.screen != "settings" {
		t.Fatal("first-run appearance skipped")
	}
	key(first, "esc")
	if first.screen != "vibe" {
		t.Fatal(first.screen)
	}
}
func TestHistorySearchPaginationDeleteAndStats(t *testing.T) {
	m := newModel(t)
	db, err := history.Open(history.Path(m.options.Home, runtime.GOOS, os.Getenv("XDG_DATA_HOME")))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for i := 0; i < 55; i++ {
		_, err := db.Record(history.Invocation{Skill: "feat", Question: "hello", AgentType: "codex"})
		if err != nil {
			t.Fatal(err)
		}
	}
	m.open("history")
	if m.total != 55 || len(m.historyRows) != 50 || !strings.Contains(m.View(), "History (55)") {
		t.Fatal(m.View())
	}
	key(m, "n")
	if len(m.historyRows) != 5 || m.offset != 50 {
		t.Fatal(m.offset)
	}
	key(m, "p")
	if m.offset != 0 {
		t.Fatal(m.offset)
	}
	key(m, "d")
	if m.total != 54 {
		t.Fatal(m.total)
	}
	key(m, "/")
	key(m, "missing")
	if len(m.historyRows) != 0 {
		t.Fatal(m.historyRows)
	}
	key(m, "esc")
	m.open("stats")
	if m.stats.Total != 54 || !strings.Contains(m.View(), "OpenAI Codex") {
		t.Fatal(m.View())
	}
	key(m, "r")
	key(m, "esc")
	if m.screen != "hub" {
		t.Fatal(m.screen)
	}
}
func TestSessionsSortingLogsAndCleanup(t *testing.T) {
	m := newModel(t)
	manager := m.manager()
	s := sessions.Session{ID: "dead1234", Skill: "feat", TmuxSession: "ashley-test-missing", LogFile: filepath.Join(manager.Dir, "dead.log"), StartedAt: time.Now().Format(time.RFC3339)}
	if err := manager.Save(s); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(s.LogFile, []byte("retained log"), 0600)
	m.open("sessions")
	if len(m.sessionRows) != 1 {
		t.Fatal(m.sessionRows)
	}
	key(m, "s")
	if m.sortMode != "skill" {
		t.Fatal(m.sortMode)
	}
	key(m, "l")
	if m.screen != "log" || !strings.Contains(m.View(), "retained log") {
		t.Fatal(m.View())
	}
	key(m, "esc")
	m.open("sessions")
	key(m, "d")
	if len(m.sessionRows) != 0 {
		t.Fatal(m.sessionRows)
	}
	if _, err := os.Stat(s.LogFile); !os.IsNotExist(err) {
		t.Fatal("deleted session log remains")
	}
	key(m, "X")
	key(m, "k")
	m.Update(tick(time.Now()))
}

func TestCreatorValidatesPreviewsAndSavesCustomSkill(t *testing.T) {
	m := newModel(t)
	m.open("create")
	m.editor.SetValue(`{"name":"a-custom","description":"My workflow","extends":"feat","components+":["components/quality-assurance.md"],"checklist":["Done"]}`)
	name, _, preview, err := m.createdDefinition()
	if err != nil || name != "custom" || !strings.Contains(preview, "Done") {
		t.Fatal(name, preview, err)
	}
	m.wizard = nil
	key(m, "ctrl+r")
	if m.screen != "create-preview" || !strings.Contains(m.logContent, "Done") {
		t.Fatal("JSON preview shortcut did not open the assembled skill")
	}
	key(m, "esc")
	m.saveCreated()
	path := filepath.Join(m.options.Home, ".ashley", "skills", "custom.jsonc")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	catalog := skills.Catalog{Source: skills.Overlay{User: os.DirFS(filepath.Join(m.options.Home, ".ashley")), Base: ashley.Assets}}
	names, err := catalog.Names()
	if err != nil || len(names) != 15 {
		t.Fatal(names, err)
	}
	result, err := catalog.Assemble("custom", nil)
	if err != nil || !strings.Contains(result.Content, "Done") {
		t.Fatal(err)
	}
	m.editor.SetValue(`{"name":"a-custom","description":"overwrite"}`)
	m.saveCreated()
	if after, _ := os.ReadFile(path); string(after) != string(data) {
		t.Fatal("existing skill overwritten")
	}
	m.editor.SetValue(`{"name":"../escape"}`)
	if _, _, _, err := m.createdDefinition(); err == nil {
		t.Fatal("unsafe name accepted")
	}
}

func TestMouseNavigationAndSettings(t *testing.T) {
	m := newModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	click := func(x, y int) {
		m.Update(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	}
	click(5, 6)
	if m.cursor != 1 {
		t.Fatal("session row not selected", m.cursor)
	}
	click(5, 6)
	if m.screen != "sessions" {
		t.Fatal(m.screen)
	}
	m.open("settings")
	clickSetting(t, m, "codex")
	if m.agent != "codex" {
		t.Fatal(m.agent)
	}
	clickSetting(t, m, "light")
	if m.theme.Mode != "light" {
		t.Fatal(m.theme)
	}
	clickSetting(t, m, "purple")
	if m.theme.Preset != "purple" {
		t.Fatal(m.theme)
	}
	m.open("vibe")
	m.Update(tea.MouseMsg{X: 2, Y: 5, Button: tea.MouseButtonWheelDown})
	if m.cursor != 1 {
		t.Fatal(m.cursor)
	}
	key(m, "/")
	if m.focus != "filter" {
		t.Fatal(m.focus)
	}
	key(m, "tab")
	if m.focus != "question" {
		t.Fatal(m.focus)
	}
	key(m, "tab")
	if m.focus != "" {
		t.Fatal(m.focus)
	}
	m.Update(tea.WindowSizeMsg{Width: 50, Height: 20})
	if m.width != 50 {
		t.Fatal(m.width)
	}
}
func TestStandaloneQuitAndLogBack(t *testing.T) {
	m := newModel(t)
	m.options.Screen = "sessions"
	m.open("sessions")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("standalone quit ignored")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("not a quit command")
	}
	m.screen = "log"
	key(m, "esc")
	if m.screen != "sessions" {
		t.Fatal(m.screen)
	}
	m.options.Screen = "history"
	m.open("history")
	m.filter.SetValue("query")
	key(m, "esc")
	if m.filter.Value() != "" || m.screen != "history" {
		t.Fatal("standalone history filter not cleared")
	}
}
func TestCreatorPreservesOmittedInheritanceFields(t *testing.T) {
	m := newModel(t)
	m.open("create")
	m.editor.SetValue(`{"name":"child","description":"Custom inherited workflow","extends":"feat","extension":{"custom":true}}`)
	_, data, _, err := m.createdDefinition()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"components":`) || strings.Contains(string(data), `"resources":`) {
		t.Fatal("omitted inheritance fields became replacements", string(data))
	}
	if !strings.Contains(string(data), `"extension"`) {
		t.Fatal("extension fields were discarded")
	}
}

func TestClipboardCopiesPromptAndReportsUnavailableCommand(t *testing.T) {
	m := newModel(t)
	bin := t.TempDir()
	output := filepath.Join(bin, "clipboard")
	t.Setenv("ASHLEY_TEST_CLIPBOARD", output)
	for _, name := range []string{"pbcopy", "wl-copy"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n/bin/cat > \"$ASHLEY_TEST_CLIPBOARD\"\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	m.open("vibe")
	m.question.SetValue("Make this work")
	cmd := m.copyPrompt()
	if cmd == nil {
		t.Fatal("no clipboard action")
	}
	message := cmd().(completed)
	if message.err != nil {
		t.Fatal(message.err)
	}
	data, err := os.ReadFile(output)
	if err != nil || !strings.Contains(string(data), "<command-args>\nMake this work\n</command-args>") {
		t.Fatal("copied prompt lost its question", err)
	}
	message = m.copyText("first line\nsecond line")().(completed)
	data, err = os.ReadFile(output)
	if message.err != nil || err != nil || string(data) != "first line\nsecond line" {
		t.Fatal("clipboard altered multiline text", message.err, err)
	}
	m.Update(message)
	if m.status != "✓ Completed successfully." {
		t.Fatal(m.status)
	}
	t.Setenv("PATH", t.TempDir())
	message = m.copyText("text")().(completed)
	if message.err == nil {
		t.Fatal("missing clipboard command ignored")
	}
	m.Update(message)
	if m.status == "Completed." {
		t.Fatal("clipboard failure hidden")
	}
}
