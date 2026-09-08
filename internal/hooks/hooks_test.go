package hooks

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LBYPatrick/ashley/internal/config"
)

func TestEnvironmentAndOrdering(t *testing.T) {
	dir := t.TempDir()
	code := 7
	c := Context{CWD: dir, Skill: "feat", Question: "a $literal `question`\nsecond line", Permission: "afk", SessionID: "abcd", ExitCode: &code}
	r := Runner{}
	err := r.Run(context.Background(), []string{`printf '%s\n' "$ASHLEY_SKILL" "$ASHLEY_QUESTION" "$ASHLEY_PERMISSION" "$ASHLEY_SESSION_ID" "$ASHLEY_EXIT_CODE" > context`, `echo next >> context`}, c)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "context"))
	if string(data) != "feat\na $literal `question`\nsecond line\nafk\nabcd\n7\nnext\n" {
		t.Fatal(string(data))
	}
	err = r.Before(context.Background(), config.Hooks{BeforeRun: []string{"false", "touch should-not-exist"}}, c)
	if err == nil {
		t.Fatal("failure ignored")
	}
	if _, err := os.Stat(filepath.Join(dir, "should-not-exist")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("ran hook after failure")
	}
	h := config.Hooks{AfterRun: []string{"false"}, OnError: []string{"touch error-hook"}}
	if err := r.After(context.Background(), h, c); err == nil {
		t.Fatal("after error lost")
	}
	if _, err := os.Stat(filepath.Join(dir, "error-hook")); err != nil {
		t.Fatal("on_error skipped", err)
	}
	code = 0
	if err := r.After(context.Background(), config.Hooks{OnError: []string{"false"}}, c); err != nil {
		t.Fatal("ran on_error after success")
	}
}
func TestTimeoutAndCancellation(t *testing.T) {
	r := Runner{Timeout: 30 * time.Millisecond}
	start := time.Now()
	err := r.Run(context.Background(), []string{"sleep 30 & wait"}, Context{CWD: t.TempDir()})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("hook child survived timeout")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := r.Run(ctx, []string{"true"}, Context{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := r.Run(context.Background(), []string{"true"}, Context{CWD: "/nonexistent/ashley-test"}); err == nil {
		t.Fatal("missing cwd ignored")
	}
}
func TestNoStaleEnvironment(t *testing.T) {
	env := (Context{Skill: "commit"}).Environment([]string{"ASHLEY_EXIT_CODE=1", "ASHLEY_SESSION_ID=old", "CUSTOM=preserved", "invalid"})
	joined := strings.Join(env, "\n")
	if strings.Contains(joined, "ASHLEY_EXIT_CODE=") || strings.Contains(joined, "ASHLEY_SESSION_ID=") || !strings.Contains(joined, "CUSTOM=preserved") || !strings.Contains(joined, "ASHLEY_PERMISSION=default") {
		t.Fatal(env)
	}
}
