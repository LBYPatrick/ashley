package execution

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LBYPatrick/ashley/internal/config"
	"github.com/LBYPatrick/ashley/internal/history"
	"github.com/LBYPatrick/ashley/internal/hooks"
)

func TestOutcomeAndHooks(t *testing.T) {
	for _, code := range []int{0, 7} {
		dir := t.TempDir()
		path := filepath.Join(dir, "history.db")
		store, err := history.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer store.Close()
		id, err := store.Record(history.Invocation{Skill: "raw"})
		if err != nil {
			t.Fatal(err)
		}
		command := "printf agent-output; exit 0"
		if code == 7 {
			command = "exit 7"
		}
		job := Job{Args: []string{"/bin/sh", "-c", command}, Context: hooks.Context{CWD: dir, Skill: "raw", SessionID: "1234"}, HistoryPath: path, InvocationID: id, Hooks: config.Hooks{AfterRun: []string{`printf '%s:%s' "$ASHLEY_EXIT_CODE" "$ASHLEY_SESSION_ID" > after`}, OnError: []string{"touch error"}}}
		file, err := SaveJob(dir, job)
		if err != nil {
			t.Fatal(err)
		}
		info, _ := os.Stat(file)
		if info.Mode().Perm() != 0600 {
			t.Fatal("job not private")
		}
		loaded, err := LoadJob(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("job not consumed")
		}
		var output bytes.Buffer
		err = Run(context.Background(), loaded, nil, &output, &output)
		if code == 0 && err != nil {
			t.Fatal(err)
		}
		if code != 0 {
			var exit ExitError
			if !errors.As(err, &exit) || exit.Code != code {
				t.Fatal(err)
			}
		}
		rows, err := store.Query(history.Filter{}, 50, 0)
		if err != nil || rows[0].ExitCode == nil || *rows[0].ExitCode != code || rows[0].DurationS == nil {
			t.Fatal(rows, err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "after"))
		if err != nil || !strings.HasSuffix(string(data), ":1234") {
			t.Fatal(string(data), err)
		}
		_, err = os.Stat(filepath.Join(dir, "error"))
		if (err == nil) != (code != 0) {
			t.Fatal("wrong error hook behavior")
		}
	}
}
func TestCancelledAgent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.db")
	store, err := history.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	id, _ := store.Record(history.Invocation{Skill: "raw"})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	var output bytes.Buffer
	err = Run(ctx, Job{Args: []string{"/bin/sleep", "30"}, HistoryPath: path, InvocationID: id}, nil, &output, &output)
	var exit ExitError
	if !errors.As(err, &exit) || exit.Code != 130 {
		t.Fatal(err)
	}
}
