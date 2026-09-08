package sessions

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestScrollingIsolation(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	socket := fmt.Sprintf("ashley-go-test-%d", time.Now().UnixNano())
	run := func(args []string, input string) (string, error) {
		return Tmux(append([]string{"-L", socket, "-f", "/dev/null"}, args...), input)
	}
	if _, err := run([]string{"new-session", "-d", "-s", "test", "sleep", "300"}, ""); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { run([]string{"kill-server"}, "") })
	before, err := run([]string{"list-keys", "-T", "root"}, "")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := ConfigureScrolling(run, "test"); err != nil {
			t.Fatal(err)
		}
	}
	after, _ := run([]string{"list-keys", "-T", "root"}, "")
	if before != after {
		t.Fatal("changed root key bindings")
	}
	mouse, _ := run([]string{"show-options", "-Av", "-t", "test", "mouse"}, "")
	if strings.TrimSpace(mouse) != "on" {
		t.Fatal("mouse is disabled")
	}
	bindings, _ := run([]string{"list-keys", "-T", "ashley-scroll-root"}, "")
	for _, line := range strings.Split(bindings, "\n") {
		if strings.Contains(line, "WheelUpPane") && (!strings.Contains(line, "copy-mode -e") || strings.Contains(line, "send-keys -M")) {
			t.Fatal(line)
		}
	}
}

func TestScrollingPropagatesFailures(t *testing.T) {
	failure := errors.New("tmux failed")
	for failAt := 1; failAt <= 7; failAt++ {
		calls := 0
		run := func(args []string, input string) (string, error) {
			calls++
			if calls == failAt {
				return "", failure
			}
			if args[0] == "show-options" {
				return "root\n", nil
			}
			return "bind-key -T root F12 display-message hello\n", nil
		}
		if err := ConfigureScrolling(run, "test"); !errors.Is(err, failure) {
			t.Fatalf("step %d: %v", failAt, err)
		}
	}
}
