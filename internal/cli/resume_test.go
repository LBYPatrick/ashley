package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/LBYPatrick/ashley/internal/agents"
	"github.com/LBYPatrick/ashley/internal/execution"
	"github.com/LBYPatrick/ashley/internal/history"
	"github.com/LBYPatrick/ashley/internal/sessions"
)

func TestHistoryResumeLaunchAndReattach(t *testing.T) {
	for _, agent := range agents.Keys() {
		t.Run(agent, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
			bin := filepath.Join(home, "bin")
			os.MkdirAll(bin, 0700)
			os.WriteFile(filepath.Join(bin, agent), []byte("#!/bin/sh\nexit 0\n"), 0700)
			os.WriteFile(filepath.Join(bin, "zellij"), []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"$HOME/zellij-args\"\ncase \"$1\" in\n--version) echo 'zellij 0.45.1';;\n--config) [ -z \"$ASHLEY_TEST_FAIL_START\" ] || exit 1; echo \"$5\" > \"$HOME/zellij-live\";;\nlist-sessions) /bin/cat \"$HOME/zellij-live\";;\nesac\n"), 0700)
			t.Setenv("PATH", bin)
			t.Setenv("ASHLEY_TEST_FAIL_START", "")
			// A changed preference must not change the original invocation's permissions.
			os.MkdirAll(filepath.Join(home, ".ashley"), 0700)
			os.WriteFile(filepath.Join(home, ".ashley/config.yaml"), []byte("defaults:\n  permission_mode: dsp\n"), 0600)
			store, err := history.Open(history.Path(home, runtime.GOOS, os.Getenv("XDG_DATA_HOME")))
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			id, err := store.Record(history.Invocation{Skill: "raw", Question: "never send this again", CWD: home, AgentType: agent, AgentSessionID: "conversation-42", Permission: "default", SessionID: "previous"})
			if err != nil {
				t.Fatal(err)
			}
			manager, err := sessions.User()
			if err != nil {
				t.Fatal(err)
			}
			logPath := filepath.Join(home, "previous.log")
			if err := os.WriteFile(logPath, []byte("previous output"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := manager.Save(sessions.Session{ID: "previous", Backend: "zellij", ZellijSession: "ashley-previous", CWD: home, LogFile: logPath}); err != nil {
				t.Fatal(err)
			}

			var out bytes.Buffer
			if err := Run([]string{"history", "resume", fmt.Sprint(id)}, &out, &out); err != nil {
				t.Fatal(err, out.String())
			}
			rows, err := store.Query(history.Filter{}, -1, 0)
			if err != nil || len(rows) != 1 {
				t.Fatal(rows, err)
			}
			if rows[0].ID != id || rows[0].SessionID == "" || rows[0].SessionID == "previous" || rows[0].Name != "" || rows[0].Question != "never send this again" || rows[0].AgentSessionID != "conversation-42" || rows[0].CWD != home || rows[0].Permission != "default" {
				t.Fatal(rows[0])
			}
			files, _ := filepath.Glob(filepath.Join(home, ".ashley/sessions/.ashley-job-*.json"))
			if len(files) != 1 {
				t.Fatal(files)
			}
			data, err := os.ReadFile(files[0])
			if err != nil {
				t.Fatal(err)
			}
			var job execution.Job
			if err := json.Unmarshal(data, &job); err != nil {
				t.Fatal(err)
			}
			want, _ := agents.ResumeArgs(agent, "conversation-42")
			if !slices.Equal(job.Args[1:], want) || job.Context.CWD != home || job.InvocationID != id || job.Context.SessionID != rows[0].SessionID {
				t.Fatal(job)
			}
			// Choosing the original row a second time attaches the already resumed instance.
			if err := Run([]string{"history", "resume", fmt.Sprint(id)}, &out, &out); err != nil {
				t.Fatal(err)
			}
			count, _ := store.Count(history.Filter{})
			if count != 1 {
				t.Fatal("duplicate conversation launched", count)
			}
			calls, _ := os.ReadFile(filepath.Join(home, "zellij-args"))
			if strings.Count(string(calls), "--create-background\n") != 1 || strings.Count(string(calls), "attach\n") != 3 {
				t.Fatal(string(calls))
			}
			if old, err := manager.Load("previous"); err != nil || !old.Superseded {
				t.Fatalf("old session metadata still visible: %v", err)
			}
			if data, err := os.ReadFile(logPath); err != nil || string(data) != "previous output" {
				t.Fatalf("old log was not preserved: %q, %v", data, err)
			}
			if err := os.Remove(filepath.Join(home, "zellij-live")); err != nil {
				t.Fatal(err)
			}
			t.Setenv("ASHLEY_TEST_FAIL_START", "1")
			if err := Run([]string{"history", "resume", fmt.Sprint(id)}, &out, &out); err == nil {
				t.Fatal("failed backend start returned success")
			}
			if row, err := store.Get(id); err != nil || row.SessionID != rows[0].SessionID {
				t.Fatalf("failed resume changed the history link: %+v, %v", row, err)
			}
			t.Setenv("ASHLEY_TEST_FAIL_START", "")
			if err := Run([]string{"history", "resume", fmt.Sprint(id)}, &out, &out); err != nil {
				t.Fatal(err, out.String())
			}
			row, err := store.Get(id)
			if err != nil || row.SessionID == rows[0].SessionID || row.SessionID == "" {
				t.Fatalf("second resume did not replace the session link: %+v, %v", row, err)
			}
			if count, err := store.Count(history.Filter{}); err != nil || count != 1 {
				t.Fatalf("second resume added a history row: %d, %v", count, err)
			}
			if old, err := manager.Load(rows[0].SessionID); err != nil || !old.Superseded {
				t.Fatalf("previous resumed session still visible: %v", err)
			}

		})
	}
}

func TestHistoryResumeUnavailableAndLegacyLive(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("PATH", home)
	store, err := history.Open(history.Path(home, runtime.GOOS, os.Getenv("XDG_DATA_HOME")))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	legacy, _ := store.Record(history.Invocation{Skill: "raw", CWD: home})
	missingDir, _ := store.Record(history.Invocation{Skill: "raw", CWD: filepath.Join(home, "missing"), AgentSessionID: "abc"})
	for _, args := range [][]string{{"history", "resume"}, {"history", "resume", "0"}, {"history", "resume", "999"}, {"history", "resume", fmt.Sprint(legacy)}, {"history", "resume", fmt.Sprint(missingDir)}} {
		var out bytes.Buffer
		if err := Run(args, &out, &out); err == nil {
			t.Fatal("accepted", args)
		}
	}
	os.WriteFile(filepath.Join(home, "tmux"), []byte("#!/bin/sh\nexit 0\n"), 0700)
	manager, _ := sessions.User()
	s := sessions.Session{ID: "legacy", TmuxSession: "ashley-legacy", CWD: home, LogFile: filepath.Join(home, "legacy.log")}
	if err := manager.Save(s); err != nil {
		t.Fatal(err)
	}
	id, _ := store.Record(history.Invocation{Skill: "raw", CWD: home, SessionID: "legacy"})
	var out bytes.Buffer
	if err := Run([]string{"history", "resume", fmt.Sprint(id)}, &out, &out); err != nil {
		t.Fatal(err)
	}
	count, _ := store.Count(history.Filter{})
	if count != 3 {
		t.Fatal(count)
	}
}
