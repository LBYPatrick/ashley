package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/LBYPatrick/ashley/internal/execution"
	"github.com/LBYPatrick/ashley/internal/history"
)

func TestRunOptions(t *testing.T) {
	o, detached, err := runOptions([]string{"raw", "question", "--grok", "--auto", "-afk", "--detached"})
	if err != nil || !detached || o.Agent != "grok" || !o.Auto || !o.AFK || o.Question != "question" {
		t.Fatal(o, detached, err)
	}
	o, _, err = runOptions([]string{"--codex", "raw", "--", "--literal", "text"})
	if err != nil || o.Question != "--literal text" {
		t.Fatal(o, err)
	}
	for _, args := range [][]string{{}, {"raw", "--unknown"}, {"raw", "--codex", "--claude"}, {"raw", "--normal", "--auto"}} {
		if _, _, err := runOptions(args); err == nil {
			t.Fatal(args)
		}
	}
}
func TestPipelineHooksHistoryAndFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	bin := filepath.Join(home, "bin")
	os.MkdirAll(bin, 0700)
	agent := filepath.Join(bin, "codex")
	os.WriteFile(agent, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"$HOME/args\"\n"), 0700)
	t.Setenv("PATH", bin)
	cfg := filepath.Join(home, ".ashley")
	os.MkdirAll(cfg, 0700)
	os.WriteFile(filepath.Join(cfg, "prefs.json"), []byte(`{"agent":"codex"}`), 0600)
	os.WriteFile(filepath.Join(cfg, "config.yaml"), []byte("defaults:\n  permission_mode: auto\nhooks:\n  global:\n    before_run: 'echo before >> hooks'\n    after_run: 'echo after >> hooks'\npipelines:\n  demo: [raw, raw]\n"), 0600)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(home); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)
	var output bytes.Buffer
	if err := Run([]string{"pipe", "demo", "hello"}, &output, &output); err != nil {
		t.Fatal(err, output.String())
	}
	data, _ := os.ReadFile(filepath.Join(home, "args"))
	if strings.Count(string(data), "hello") != 1 || strings.Count(string(data), "workspace-write") != 2 {
		t.Fatal(string(data))
	}
	hooks, _ := os.ReadFile(filepath.Join(home, "hooks"))
	if string(hooks) != "before\nafter\nbefore\nafter\n" {
		t.Fatal(string(hooks))
	}
	store, err := history.Open(history.Path(home, runtime.GOOS, os.Getenv("XDG_DATA_HOME")))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	rows, err := store.Query(history.Filter{}, 50, 0)
	if err != nil || len(rows) != 2 {
		t.Fatal(rows, err)
	}
	if !reflect.DeepEqual([]string{rows[0].Question, rows[1].Question}, []string{"", "hello"}) || rows[0].Outcome != "success" {
		t.Fatal(rows)
	}
	if err := os.Remove(filepath.Join(home, "args")); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"pipe", "raw", "--normal", "normal question"}, &output, &output); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(filepath.Join(home, "args"))
	if strings.Contains(string(data), "workspace-write") || !strings.Contains(string(data), "normal question") {
		t.Fatalf("normal inherited configured auto mode: %s", data)
	}
	rows, err = store.Query(history.Filter{}, 1, 0)
	if err != nil || len(rows) != 1 || rows[0].Permission != "default" {
		t.Fatal(rows, err)
	}
	os.WriteFile(agent, []byte("#!/bin/sh\nexit 9\n"), 0700)
	err = Run([]string{"pipe", "demo"}, &output, &output)
	var exit execution.ExitError
	if !errors.As(err, &exit) || exit.Code != 9 {
		t.Fatal(err)
	}
	count, err := store.Count(history.Filter{})
	if err != nil || count != 4 {
		t.Fatal("did not stop at failed step", count, err)
	}
	os.WriteFile(filepath.Join(cfg, "config.yaml"), []byte("hooks:\n  global:\n    before_run: 'exit 3'\n"), 0600)
	if err := Run([]string{"pipe", "raw"}, &output, &output); err == nil || !strings.Contains(err.Error(), "before_run") {
		t.Fatal(err)
	}
	count, _ = store.Count(history.Filter{})
	if count != 4 {
		t.Fatal("recorded blocked invocation")
	}
}
