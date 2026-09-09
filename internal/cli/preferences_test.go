package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/LBYPatrick/ashley/internal/history"
)

func TestPreferencesCommands(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("EDITOR", "")
	for _, key := range []string{"claude", "codex", "grok", "opencode", "kilo"} {
		var out bytes.Buffer
		if err := Run([]string{"agent", key}, &out, &out); err != nil {
			t.Fatal(err)
		}
		out.Reset()
		if err := Run([]string{"agent"}, &out, &out); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "("+key+")") {
			t.Fatal(out.String())
		}
	}
	var out bytes.Buffer
	if err := Run([]string{"config"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".ashley", "config.yaml")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", `printf 'editor path: %s\n'`)
	if err := Run([]string{"config"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "editor path: "+path) {
		t.Fatal(out.String())
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("overwrote config")
	}
	for _, args := range [][]string{{"agent", "invalid"}, {"agent", "codex", "claude"}, {"config", "invalid"}} {
		if err := Run(args, &out, &out); err == nil {
			t.Fatal(args)
		}
	}
}
func TestHistoryCommands(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	store, err := history.Open(history.Path(home, runtime.GOOS, os.Getenv("XDG_DATA_HOME")))
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Record(history.Invocation{Skill: "feat", Question: "CLI record", AgentType: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	store.Close()
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"history"}, "CLI record"},
		{[]string{"history", "show", "--agent", "codex", "--json"}, `"agent_type":"codex"`},
		{[]string{"history", "show", "--skill", "missing"}, "No history"},
		{[]string{"history", "stats"}, "Invocations  1"},
		{[]string{"history", "stats", "--json"}, `"total":1`},
		{[]string{"history", "info"}, "Entries      1"},
		{[]string{"history", "prune", "30"}, "Pruned 0 entries"},
		{[]string{"history", "clear", "--yes"}, "Cleared 1 history entries"},
	} {
		var out bytes.Buffer
		if err := Run(tc.args, &out, &out); err != nil {
			t.Fatal(tc.args, err)
		}
		if !strings.Contains(out.String(), tc.want) {
			t.Fatal(tc.args, out.String())
		}
	}
	for _, args := range [][]string{{"history", "prune", "-1"}, {"history", "prune"}, {"history", "show", "--offset", "-1"}, {"history", "stats", "--agent", "unknown"}, {"history", "info", "extra"}, {"history", "unknown"}} {
		var out bytes.Buffer
		if err := Run(args, &out, &out); err == nil {
			t.Fatal(args)
		}
	}
}

func TestBinarySkillInstallationCommands(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, key := range []string{"CLAUDE_CONFIG_DIR", "CODEX_HOME", "GROK_HOME", "OPENCODE_CONFIG_DIR", "XDG_CONFIG_HOME"} {
		t.Setenv(key, "")
	}
	var out bytes.Buffer
	if err := Run([]string{"install", "--all", "--skills-only"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "14 skills for 5 agents") {
		t.Fatal(out.String())
	}
	if _, err := os.ReadFile(filepath.Join(home, ".codex", "skills", "a-feat", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"uninstall"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(home, ".codex", "skills", "a-feat")); !os.IsNotExist(err) {
		t.Fatal("link remains")
	}
	for _, args := range [][]string{{"install", "--unknown"}, {"install", "--agent"}, {"install", "--check"}, {"upgrade", "--skills-only"}} {
		if err := Run(args, &out, &out); err == nil {
			t.Fatal(args)
		}
	}
}
