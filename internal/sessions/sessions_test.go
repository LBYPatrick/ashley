package sessions

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFilterRows(t *testing.T) {
	input := []Session{
		{ID: "abc123", Name: "API work", Question: "Update handlers", Skill: "feat", Agent: "codex", CWD: "/Work/Project_100%/api"},
		{ID: "def456", Name: "Web work", Question: "API work", Skill: "fix", Agent: "claude", CWD: "/Work/Project_100%/web"},
		{ID: "legacy789", Skill: "commit", CWD: "/Work/ProjectX100Y/api"},
	}
	for _, tc := range []struct {
		name   string
		filter Filter
		want   []string
	}{
		{"all", Filter{}, []string{"abc123", "def456", "legacy789"}},
		{"directory", Filter{CWD: "project_100%"}, []string{"abc123", "def456"}},
		{"combined", Filter{Search: "API WORK", CWD: "/API"}, []string{"abc123"}},
		{"name", Filter{Search: "web work"}, []string{"def456"}},
		{"question", Filter{Search: "handlers"}, []string{"abc123"}},
		{"skill", Filter{Search: "commit"}, []string{"legacy789"}},
		{"agent", Filter{Search: "CODEX"}, []string{"abc123"}},
		{"default agent", Filter{Search: "claude"}, []string{"def456", "legacy789"}},
		{"id", Filter{Search: "DEF456"}, []string{"def456"}},
		{"search directory", Filter{Search: "100%/web"}, []string{"def456"}},
		{"literal underscore", Filter{CWD: "_"}, []string{"abc123", "def456"}},
		{"literal percent", Filter{CWD: "%"}, []string{"abc123", "def456"}},
		{"blank", Filter{Search: " \t", CWD: " \t"}, []string{"abc123", "def456", "legacy789"}},
		{"directory excludes question", Filter{CWD: "handlers"}, []string{}},
		{"no matches", Filter{Search: "absent"}, []string{}},
		{"combined no matches", Filter{Search: "codex", CWD: "/web"}, []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := []string{}
			for _, row := range FilterRows(input, tc.filter) {
				got = append(got, row.ID)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("FilterRows = %v, want %v", got, tc.want)
			}
		})
	}
	filtered := FilterRows(input, Filter{})
	filtered[0].Name = "Changed"
	if input[0].Name != "API work" {
		t.Fatal("FilterRows changed the input")
	}
	if got := FilterRows(nil, Filter{}); got == nil || len(got) != 0 {
		t.Fatalf("FilterRows(nil) = %v; want an empty slice", got)
	}
	if got := FilterRows([]Session{{CWD: "/Work/ÜBER/Änderungen"}}, Filter{CWD: "/über/änderungen"}); len(got) != 1 {
		t.Fatalf("Unicode directory filter = %v; want one session", got)
	}
}

func TestLegacyMetadataAndSorting(t *testing.T) {
	m := Manager{Backend: "tmux", Dir: t.TempDir()}
	legacy := `{"id":"abc1","skill":"feat","question":"old","tmux_session":"ashley-abc1","log_file":"/tmp/old.log","started_at":"2024-01-01T00:00:00","cwd":"/tmp"}`
	if err := os.WriteFile(filepath.Join(m.Dir, "abc1.json"), []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(m.Dir, "broken.json"), []byte("{broken"), 0600)
	old, err := m.Load("abc1")
	if err != nil || old.Agent != "" {
		t.Fatal(old, err)
	}
	newer := old
	newer.ID = "abc2"
	newer.Skill = "Commit"
	newer.StartedAt = "2025-01-01T00:00:00Z"
	if err := m.Save(newer); err != nil {
		t.Fatal(err)
	}
	all, err := m.All()
	if err != nil || len(all) != 2 || all[0].ID != "abc2" {
		t.Fatal(all, err)
	}
	if sorted := Sort(all, "skill"); sorted[0].Skill != "Commit" {
		t.Fatal(sorted)
	}
	if _, err := m.Resolve("abc"); err == nil {
		t.Fatal("ambiguous prefix accepted")
	}
	if s, err := m.Resolve("abc1"); err != nil || s.ID != "abc1" {
		t.Fatal(s, err)
	}
	if _, err := m.Resolve("unknown"); err == nil {
		t.Fatal("missing session found")
	}
	if _, err := m.Load("../escape"); err == nil {
		t.Fatal("path escape accepted")
	}
	if err := m.Save(Session{ID: "../escape"}); err == nil {
		t.Fatal("path escape accepted")
	}
	if old.Elapsed(time.Date(2024, 1, 1, 1, 2, 0, 0, time.UTC)) != "1h 2m" {
		t.Fatal(old.Elapsed(time.Now()))
	}
}
func TestShellQuotingAndPromptFiles(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "prompt's file")
	question := "literal $HOME `touch bad` ' quote\nsecond line"
	os.WriteFile(file, []byte(question), 0600)
	command, files := ShellCommand([]string{"printf", "%s", PromptFileMarker + file}, dir)
	if len(files) != 1 || files[0] != file {
		t.Fatal(files)
	}
	cmd := exec.Command("bash", "-c", command)
	cmd.Stdin = strings.NewReader("\n")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err, string(output))
	}
	if !strings.HasPrefix(string(output), question) {
		t.Fatal(string(output))
	}
	if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("prompt not cleaned")
	}
	if _, err := os.Stat(filepath.Join(dir, "bad")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("question executed")
	}
}

func TestElapsed(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct{ started, want string }{
		{"2026-10-15T11:59:15Z", "45s"},
		{"2026-10-15T11:57:30Z", "2m 30s"},
		{"2026-10-15T09:55:00Z", "2h 5m"},
		{"2026-10-15T11:59:15", "45s"},
		{"2026-10-15T12:01:00Z", "0s"},
		{"invalid", "?"},
	} {
		s := Session{StartedAt: test.started}
		if got := s.Elapsed(now); got != test.want {
			t.Errorf("Elapsed(%q) = %q, want %q", test.started, got, test.want)
		}
	}
}
func TestRealSessionLifecycle(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	socket := fmt.Sprintf("ashley-session-test-%d", time.Now().UnixNano())
	run := func(args []string, input string) (string, error) {
		return Tmux(append([]string{"-L", socket, "-f", "/dev/null"}, args...), input)
	}
	t.Cleanup(func() { run([]string{"kill-server"}, "") })
	m := Manager{Backend: "tmux", Dir: t.TempDir(), Run: run}
	s, err := m.Create(Session{Skill: "raw", CWD: t.TempDir(), Agent: "codex"}, []string{"printf", "first output\nworking\r\x1b[2Kcomplete\nlast line\n"})
	if err != nil {
		t.Fatal(err)
	}
	if !m.Alive(s) {
		t.Fatal("session exited before attachment")
	}
	deadline := time.Now().Add(3 * time.Second)
	var log string
	for time.Now().Before(deadline) {
		log, err = ReadLog(s, 0)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(log, "last line") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !strings.Contains(log, "first output") || !strings.Contains(log, "last line") {
		t.Fatal("lost startup log", log)
	}
	preview, err := m.Preview(s, 50, true)
	if err != nil || !strings.Contains(preview, "complete") || strings.Contains(preview, "working") {
		t.Fatal("live preview did not capture the rendered terminal", preview, err)
	}
	if _, err := m.Resolve(s.ID[:4]); err != nil {
		t.Fatal(err)
	}
	if err := m.Kill(s); err != nil {
		t.Fatal(err)
	}
	if m.Alive(s) {
		t.Fatal("session survived kill")
	}
	if _, err := os.Stat(s.LogFile); err != nil {
		t.Fatal("log removed")
	}
	if all, err := m.All(); err != nil || len(all) != 0 {
		t.Fatal(all, err)
	}
}
func TestLogsAndDeadCleanup(t *testing.T) {
	m := Manager{Backend: "tmux", Dir: t.TempDir(), Run: func([]string, string) (string, error) { return "", fmt.Errorf("not running") }}
	s := Session{ID: "dead", TmuxSession: "ashley-dead", LogFile: filepath.Join(m.Dir, "dead.log")}
	if text, err := ReadLog(s, 10); err != nil || text != "(no log file)" {
		t.Fatal(text, err)
	}
	os.WriteFile(s.LogFile, []byte("first\nsecond\nlast\n"), 0600)
	if text, err := ReadLog(s, 2); err != nil || text != "second\nlast\n" {
		t.Fatal(text, err)
	}
	if err := m.Save(s); err != nil {
		t.Fatal(err)
	}
	if n, err := m.CleanDead(); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	m.Save(s)
	if n, err := m.KillAll(); err != nil || n != 0 {
		t.Fatal(n, err)
	}
}
