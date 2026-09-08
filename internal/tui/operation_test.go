package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestSyncRetainsResultAndDoesNotLeaveTerminal(t *testing.T) {
	m := newModel(t)
	stubSyncAgents(t, m)
	m.options.Root = t.TempDir()
	var got []string
	m.options.Background = func(_ context.Context, args []string) (string, error) {
		got = args
		return "generated/a-feat/SKILL.md\ngenerated/a-debug/SKILL.md\n", nil
	}
	m.cursor = 3
	cmd := m.activate()
	if m.screen != "sync" || m.job == nil || !m.job.busy {
		t.Fatal("missing busy result page")
	}
	if m.startOperation("sync") != nil {
		t.Fatal("duplicate work started")
	}
	m.Update(cmd())
	if !reflect.DeepEqual(got, []string{"--root", m.options.Root, "install", "--skills-only", "--agent", "claude", "--agent", "codex"}) {
		t.Fatal(got)
	}
	text := ansi.Strip(m.View())
	for _, value := range []string{"Skills synced", "Destination", "generated/a-feat/SKILL.md", "generated/a-debug/SKILL.md"} {
		if !strings.Contains(text, value) {
			t.Fatal("completion disappeared", value, text)
		}
	}
	m.open("hub")
	m.cursor = 3
	cmd = m.activate()
	if cmd == nil || !m.job.busy {
		t.Fatal("reopening Sync did not start fresh work")
	}
	m.Update(cmd())
	if !strings.Contains(ansi.Strip(m.View()), "Skills synced") {
		t.Fatal("result did not persist")
	}
	exportRegressionView(t, "sync-result", m.View())
}
func TestBackgroundFailureAndCompletionAfterNavigation(t *testing.T) {
	m := newModel(t)
	stubSyncAgents(t, m)
	m.open("sync")
	m.options.Background = func(context.Context, []string) (string, error) {
		return "Cannot read definition", errors.New("invalid skill")
	}
	cmd := m.startOperation("sync")
	m.open("hub")
	m.Update(cmd())
	if m.screen != "hub" || m.job.busy || m.job.err == nil {
		t.Fatal("background work stole navigation")
	}
	m.open("sync")
	if !strings.Contains(ansi.Strip(m.View()), "invalid skill") {
		t.Fatal("error vanished")
	}
	m.options.Background = func(context.Context, []string) (string, error) { return "ready", nil }
	m.Update(m.operationKey("r")())
	if m.job.err != nil {
		t.Fatal("retry failed")
	}
}
func TestSyncUsesGenerateThenInstallPipeline(t *testing.T) {
	m := newModel(t)
	stubSyncAgents(t, m)
	m.open("sync")
	var got []string
	m.options.Background = func(_ context.Context, args []string) (string, error) {
		got = args
		return "Installed 70 skill links; preserved 0 existing paths.", nil
	}
	cmd := m.operationKey("enter")
	m.Update(cmd())
	if !reflect.DeepEqual(got, []string{"install", "--skills-only", "--agent", "claude", "--agent", "codex"}) {
		t.Fatal(got)
	}
	if !strings.Contains(ansi.Strip(m.View()), "Skills synced for Claude Code, OpenAI Codex") {
		t.Fatal("missing install result")
	}
	exportRegressionView(t, "sync-detected-result", m.View())
}
func TestQuitCancelsSync(t *testing.T) {
	m := newModel(t)
	stubSyncAgents(t, m)
	m.open("sync")
	m.options.Background = func(ctx context.Context, _ []string) (string, error) { <-ctx.Done(); return "", ctx.Err() }
	cmd := m.startOperation("sync")
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	message := cmd().(operationFinished)
	if !errors.Is(message.err, context.Canceled) {
		t.Fatal("quit left the operation running")
	}
}
func TestHelpRestoresDraftAndPaletteScroll(t *testing.T) {
	m := newModel(t)
	m.open("create")
	wizardText(m, "unfinished")
	draft := m.wizard
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	key(m, "keys")
	key(m, "enter")
	if m.screen != "help" {
		t.Fatal("help missing")
	}
	key(m, "esc")
	if m.screen != "create" || m.wizard != draft || m.wizard.input.Value() != "unfinished" {
		t.Fatal("help discarded draft")
	}
	m.Update(tea.WindowSizeMsg{Width: 50, Height: 20})
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	for range 12 {
		key(m, "down")
	}
	if !strings.Contains(ansi.Strip(m.View()), "› Quit") {
		t.Fatal("last command hidden")
	}
	exportRegressionView(t, "palette", m.View())
}

func TestSyncWithoutDetectedAgentsDoesNotRun(t *testing.T) {
	m := newModel(t)
	t.Setenv("HOME", m.options.Home)
	t.Setenv("PATH", t.TempDir())
	t.Setenv("GROK_BIN_DIR", t.TempDir())
	m.open("sync")
	if cmd := m.operationKey("enter"); cmd != nil || m.job != nil {
		t.Fatal("installation started without detected agents")
	}
	if !strings.Contains(m.status, "No supported agents detected") {
		t.Fatal(m.status)
	}
}

func stubSyncAgents(t *testing.T, m *Model) {
	t.Helper()
	t.Setenv("HOME", m.options.Home)
	t.Setenv("PATH", t.TempDir())
	t.Setenv("GROK_BIN_DIR", t.TempDir())
	bin := filepath.Join(m.options.Home, ".local", "bin")
	if err := os.MkdirAll(bin, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"claude", "codex"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("unused"), 0755); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSyncRetainsFullLogAndScrollsToBothEnds(t *testing.T) {
	m := newModel(t)
	stubSyncAgents(t, m)
	m.open("sync")
	log := "FIRST ENTRY\n" + strings.Repeat("Generated a skill file\n", 7000) + "LAST ENTRY"
	m.options.Background = func(context.Context, []string) (string, error) { return log, nil }
	m.Update(m.startOperation("sync")())
	if m.job.output != log {
		t.Fatal("full log was truncated")
	}
	m.operationKey("end")
	if !strings.Contains(ansi.Strip(m.View()), "LAST ENTRY") {
		t.Fatal("end of log unreachable")
	}
	m.operationKey("home")
	if !strings.Contains(ansi.Strip(m.View()), "FIRST ENTRY") {
		t.Fatal("start of log unreachable")
	}
}
