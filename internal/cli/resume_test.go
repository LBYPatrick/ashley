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
			os.WriteFile(filepath.Join(bin, "tmux"), []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"$HOME/tmux-args\"\n"), 0700)
			t.Setenv("PATH", bin)
			// A changed preference must not change the original invocation's permissions.
			os.MkdirAll(filepath.Join(home, ".ashley"), 0700)
			os.WriteFile(filepath.Join(home, ".ashley/config.yaml"), []byte("defaults:\n  permission_mode: dsp\n"), 0600)
			store, err := history.Open(history.Path(home, runtime.GOOS, os.Getenv("XDG_DATA_HOME")))
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			id, err := store.Record(history.Invocation{Skill: "raw", Question: "never send this again", CWD: home, AgentType: agent, AgentSessionID: "conversation-42", Permission: "default"})
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err := Run([]string{"history", "resume", fmt.Sprint(id)}, &out, &out); err != nil {
				t.Fatal(err, out.String())
			}
			rows, err := store.Query(history.Filter{}, -1, 0)
			if err != nil || len(rows) != 2 {
				t.Fatal(rows, err)
			}
			if rows[0].AgentSessionID != "conversation-42" || rows[0].CWD != home || rows[0].Permission != "default" {
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
			if !slices.Equal(job.Args[1:], want) || job.Context.CWD != home {
				t.Fatal(job)
			}
			// Choosing the original row a second time attaches the already resumed instance.
			if err := Run([]string{"history", "resume", fmt.Sprint(id)}, &out, &out); err != nil {
				t.Fatal(err)
			}
			count, _ := store.Count(history.Filter{})
			if count != 2 {
				t.Fatal("duplicate conversation launched", count)
			}
			calls, _ := os.ReadFile(filepath.Join(home, "tmux-args"))
			if strings.Count(string(calls), "new-session\n") != 1 || strings.Count(string(calls), "attach-session\n") != 2 {
				t.Fatal(string(calls))
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
