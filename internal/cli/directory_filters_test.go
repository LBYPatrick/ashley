package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/LBYPatrick/ashley/internal/history"
	"github.com/LBYPatrick/ashley/internal/sessions"
)

func TestHistoryDirectoryFlags(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	store, err := history.Open(history.Path(home, runtime.GOOS, os.Getenv("XDG_DATA_HOME")))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []history.Invocation{
		{Skill: "feat", AgentType: "codex", Question: "API change", CWD: "/Work/Project_100%/api"},
		{Skill: "fix", AgentType: "claude", CWD: "/other"},
	} {
		if _, err := store.Record(row); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"history", "show", "--cwd", "project_100%", "--search", "API", "--agent", "codex", "--skill", "feat", "--json"}, `"cwd":"/Work/Project_100%/api"`},
		{[]string{"history", "show", "--cwd=/missing", "--json"}, "[]"},
		{[]string{"history", "stats", "--cwd", "PROJECT_100%", "--agent", "codex", "--json"}, `"total":1`},
		{[]string{"history", "stats", "--cwd=/missing", "--json"}, `"total":0`},
	} {
		var out bytes.Buffer
		if err := Run(tc.args, &out, &out); err != nil {
			t.Fatal(tc.args, err, out.String())
		}
		if !strings.Contains(out.String(), tc.want) {
			t.Fatalf("%v = %q; want %q", tc.args, out.String(), tc.want)
		}
	}
}

func TestSessionDirectoryFlags(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	manager, err := sessions.User()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []sessions.Session{
		{ID: "api1", Name: "API work", Skill: "feat", Agent: "codex", CWD: "/Work/Project_100%/api"},
		{ID: "web2", Name: "Web work", Skill: "fix", Agent: "claude", CWD: "/Work/Project_100%/web"},
		{ID: "other3", Name: "Other work", Skill: "feat", Agent: "codex", CWD: "/other"},
	} {
		row.TmuxSession = "ashley-" + row.ID
		row.LogFile = filepath.Join(home, row.ID+".log")
		if err := manager.Save(row); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		args []string
		want []string
	}{
		{[]string{"sessions", "--cwd", "PROJECT_100%", "--search", "CODEX", "--json"}, []string{"api1"}},
		{[]string{"sessions", "--cwd=project_100%", "--sort=skill", "--json"}, []string{"api1", "web2"}},
		{[]string{"sessions", "--search=/other", "--json"}, []string{"other3"}},
		{[]string{"sessions", "--cwd=/missing", "--json"}, []string{}},
	} {
		var out bytes.Buffer
		if err := Run(tc.args, &out, &out); err != nil {
			t.Fatal(tc.args, err, out.String())
		}
		var rows []sessions.Session
		if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
			t.Fatal(err, out.String())
		}
		if len(rows) != len(tc.want) {
			t.Fatalf("%v = %v; want IDs %v", tc.args, rows, tc.want)
		}
		for i, row := range rows {
			if row.ID != tc.want[i] {
				t.Fatalf("%v = %v; want IDs %v", tc.args, rows, tc.want)
			}
		}
	}
	var out bytes.Buffer
	if err := Run([]string{"sessions", "--list", "--cwd", "project_100%/api"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Directory") || !strings.Contains(out.String(), "/Work/Project_100%/api") || strings.Contains(out.String(), "web2") {
		t.Fatal(out.String())
	}
	out.Reset()
	if err := Run([]string{"sessions", "--cwd=/missing"}, &out, &out); err != nil || !strings.Contains(out.String(), "No sessions match") {
		t.Fatal(err, out.String())
	}
	for _, args := range [][]string{
		{"sessions", "--cwd"}, {"sessions", "--search"}, {"sessions", "--sort=invalid"}, {"sessions", "unexpected"},
	} {
		if err := Run(args, &out, &out); err == nil {
			t.Fatalf("accepted invalid arguments %v", args)
		}
	}
}

func TestSessionCleanRejectsFiltersWithoutDeletingRecords(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())
	manager, err := sessions.User()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []sessions.Session{
		{ID: "one", Name: "First task", CWD: "/one"},
		{ID: "two", Name: "Second task", CWD: "/two"},
	} {
		row.TmuxSession = "ashley-" + row.ID
		row.LogFile = filepath.Join(home, row.ID+".log")
		if err := manager.Save(row); err != nil {
			t.Fatal(err)
		}
		if manager.Alive(row) {
			t.Fatalf("session %s must be exited", row.ID)
		}
	}
	for _, args := range [][]string{
		{"sessions", "--clean", "--cwd", "/one"},
		{"sessions", "--clean", "--search", "First task"},
	} {
		var out bytes.Buffer
		err := Run(args, &out, &out)
		if err == nil || !strings.Contains(err.Error(), "--clean cannot be combined") {
			t.Fatalf("%v returned %v; want a filter conflict", args, err)
		}
		for _, id := range []string{"one", "two"} {
			if _, err := manager.Load(id); err != nil {
				t.Fatalf("%v deleted session %s: %v", args, id, err)
			}
		}
	}
}
