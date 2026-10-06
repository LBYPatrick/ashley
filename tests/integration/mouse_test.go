//go:build integration && (darwin || linux)

package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LBYPatrick/ashley/internal/sessions"
)

// Exercise real terminal mouse reports, rather than just inspecting bindings.
func TestAgentMouseClipboardAndPaste(t *testing.T) {
	tmux, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is not installed")
	}
	for _, mouse := range []bool{true, false} {
		t.Run(fmt.Sprintf("agent-mouse-%t", mouse), func(t *testing.T) {
			s := newSandbox(t)
			socket := fmt.Sprintf("ashley-mouse-%d", time.Now().UnixNano())
			base := []string{"-L", socket, "-f", "/dev/null"}
			run := func(args []string, input string) (string, error) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				cmd := s.command(ctx, tmux, append(append([]string{}, base...), args...)...)
				cmd.Stdin = strings.NewReader(input)
				out, err := cmd.CombinedOutput()
				return string(out), err
			}
			must := func(args ...string) string {
				t.Helper()
				out, err := run(args, "")
				if err != nil {
					t.Fatalf("tmux %v: %v: %s", args, err, out)
				}
				return out
			}
			script := "#!/bin/sh\nstty raw -echo\nwhile [ ! -f ready ]; do sleep 0.05; done\n"
			if mouse {
				script += "printf '\033[?1000h\033[?1006h'\n"
			} else {
				script += "i=0; while [ $i -lt 100 ]; do printf 'scrollback %s\\r\\n' \"$i\"; i=$((i+1)); done\n"
			}
			script += "printf '\033[?2004h\033]52;c;YXNobGV5LWNsaXBib2FyZA==\007READY\\r\\n'\ncat > received\n"
			write(t, filepath.Join(s.home, "pane.sh"), script, 0700)
			must("new-session", "-d", "-s", "test", "-c", s.home, "sh pane.sh")
			t.Cleanup(func() { run([]string{"kill-server"}, "") })
			if err := sessions.ConfigureScrolling(run, "test"); err != nil {
				t.Fatal(err)
			}
			p := startTerminal(t, s, tmux, append(base, "attach-session", "-t", "test")...)
			p.waitFor("\x1b[?1006h")
			write(t, filepath.Join(s.home, "ready"), "", 0600)
			p.waitFor("READY")
			if got := must("show-buffer"); strings.TrimSpace(got) != "ashley-clipboard" {
				t.Fatalf("application clipboard request lost: %q", got)
			}
			p.waitFor("52;") // tmux also forwards the selection to the outer terminal.
			up, down := "\x1b[<64;10;10M", "\x1b[<65;10;10M"
			p.send(up)
			if mouse {
				p.send(down)
			}
			if !mouse {
				p.until(func() bool {
					return strings.TrimSpace(must("display-message", "-p", "-t", "test", "#{pane_in_mode}")) == "1"
				})
				p.send("q")
				p.until(func() bool {
					return strings.TrimSpace(must("display-message", "-p", "-t", "test", "#{pane_in_mode}")) == "0"
				})
			}
			paste := "\x1b[200~pasted text\x1b[201~"
			p.send(paste)
			p.until(func() bool {
				data, _ := os.ReadFile(filepath.Join(s.home, "received"))
				return strings.Contains(string(data), paste)
			})
			got := read(t, filepath.Join(s.home, "received"))
			if mouse && (!strings.Contains(got, up) || !strings.Contains(got, down)) {
				t.Fatalf("agent did not receive both wheel directions: %q", got)
			}
			if mode := must("display-message", "-p", "-t", "test", "#{pane_in_mode}"); strings.TrimSpace(mode) != "0" {
				t.Fatal("agent is stuck in copy mode")
			}
		})
	}
}
