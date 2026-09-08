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

func TestGenerationRetainsResultAndDoesNotLeaveTerminal(t *testing.T) {
	m := newModel(t)
	m.options.Root = t.TempDir()
	var got []string
	m.options.Background = func(_ context.Context, args []string) (string, error) {
		got = args
		return "generated/a-feat/SKILL.md\ngenerated/a-debug/SKILL.md\n", nil
	}
	m.cursor = 3
	cmd := m.activate()
	if m.screen != "generate" || m.job == nil || !m.job.busy {
		t.Fatal("missing busy result page")
	}
	if m.startOperation("generate") != nil {
		t.Fatal("duplicate work started")
	}
	m.Update(cmd())
	if !reflect.DeepEqual(got, []string{"--root", m.options.Root, "generate", "--output", m.options.Root}) {
		t.Fatal(got)
	}
	text := ansi.Strip(m.View())
	for _, value := range []string{"Skills generated", "Destination", "generated/a-feat/SKILL.md", "generated/a-debug/SKILL.md"} {
		if !strings.Contains(text, value) {
			t.Fatal("completion disappeared", value, text)
		}
	}
	m.open("hub")
	m.cursor = 3
	if cmd := m.activate(); cmd != nil {
		t.Fatal("reopening a result reran generation")
	}
	if !strings.Contains(ansi.Strip(m.View()), "Skills generated") {
		t.Fatal("result did not persist")
	}
	exportRegressionView(t, "generate-result", m.View())
}
func TestBackgroundFailureAndCompletionAfterNavigation(t *testing.T) {
	m := newModel(t)
	m.open("generate")
	m.options.Background = func(context.Context, []string) (string, error) {
		return "Cannot read definition", errors.New("invalid skill")
	}
	cmd := m.startOperation("generate")
	m.open("hub")
	m.Update(cmd())
	if m.screen != "hub" || m.job.busy || m.job.err == nil {
		t.Fatal("background work stole navigation")
	}
	m.open("generate")
	if !strings.Contains(ansi.Strip(m.View()), "invalid skill") {
		t.Fatal("error vanished")
	}
	m.options.Background = func(context.Context, []string) (string, error) { return "ready", nil }
	m.Update(m.operationKey("r")())
	if m.job.err != nil {
		t.Fatal("retry failed")
	}
}
func TestInstallScreenUsesEmbeddedSkillsOnlyCommand(t *testing.T) {
	m := newModel(t)
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
	m.open("install")
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
	if !strings.Contains(ansi.Strip(m.View()), "Skills installed for Claude Code, OpenAI Codex") {
		t.Fatal("missing install result")
	}
	exportRegressionView(t, "install-result", m.View())
}
func TestBackgroundOutputIsBoundedAndQuitCancels(t *testing.T) {
	var tail outputTail
	input := strings.Repeat("x", 200000) + "TAIL"
	n, err := tail.Write([]byte(input))
	if err != nil || n != len(input) || len(tail.data) != 128*1024 || !strings.HasSuffix(string(tail.data), "TAIL") {
		t.Fatal("unbounded or lost output")
	}
	m := newModel(t)
	m.open("generate")
	m.options.Background = func(ctx context.Context, _ []string) (string, error) { <-ctx.Done(); return "", ctx.Err() }
	cmd := m.startOperation("generate")
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

func TestInstallWithoutDetectedAgentsDoesNotRun(t *testing.T) {
	m := newModel(t)
	t.Setenv("HOME", m.options.Home)
	t.Setenv("PATH", t.TempDir())
	t.Setenv("GROK_BIN_DIR", t.TempDir())
	m.open("install")
	if cmd := m.operationKey("enter"); cmd != nil || m.job != nil {
		t.Fatal("installation started without detected agents")
	}
	if !strings.Contains(m.status, "No supported agents detected") {
		t.Fatal(m.status)
	}
}
