//go:build integration && (darwin || linux)

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LBYPatrick/ashley/internal/sessions"
	"github.com/creack/pty"
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
			if mouse && (!strings.Contains(got, "\x1b[<64;") || !strings.Contains(got, "\x1b[<65;")) {
				t.Fatalf("agent did not receive both wheel directions: %q", got)
			}
			if mode := must("display-message", "-p", "-t", "test", "#{pane_in_mode}"); strings.TrimSpace(mode) != "0" {
				t.Fatal("agent is stuck in copy mode")
			}
		})
	}
}

// Check the Ashley supervisor and Zellij together, including the PTY boundary.
func TestZellijRemoteInputDetachAndResize(t *testing.T) {
	if _, err := exec.LookPath("zellij"); err != nil {
		t.Skip("zellij not installed")
	}
	for _, scenario := range []struct{ mouse, mosh bool }{{true, false}, {false, false}, {true, true}} {
		t.Run(fmt.Sprintf("mouse=%t/mosh=%t", scenario.mouse, scenario.mosh), func(t *testing.T) {
			mouse := scenario.mouse
			if scenario.mosh {
				if _, err := exec.LookPath("mosh-client"); err != nil {
					t.Skip("mosh not installed")
				}
			}
			s := newSandbox(t)
			s.env["SSH_CONNECTION"] = "192.0.2.1 5000 192.0.2.2 22"
			tools := filepath.Join(s.home, "tools")
			script := "#!/bin/sh\nstty raw -echo\nwhile [ ! -f ready ]; do sleep 0.05; done\n"
			if mouse {
				script += "printf '\033[?1000h\033[?1006h'\n"
			} else {
				script += "i=0; while [ $i -lt 100 ]; do printf 'scrollback %s\\r\\n' \"$i\"; i=$((i+1)); done\n"
			}
			script += "printf '\033[?2004h\033]52;c;YXNobGV5LWNsaXBib2FyZA==\007READY\\r\\n'\ncat > received\n"
			write(t, filepath.Join(tools, "claude"), script, 0700)
			s.env["PATH"] = tools + ":/usr/bin:/bin"
			s.must(binary, "run", "--claude", "--detached", "--name", "Remote test", "raw", "test")
			var rows []sessions.Session
			if err := json.Unmarshal([]byte(s.must(binary, "sessions", "--json")), &rows); err != nil || len(rows) != 1 {
				t.Fatal(rows, err)
			}
			session := rows[0]
			if session.Backend != "zellij" || session.Name != "Remote test" {
				t.Fatal(session)
			}
			attach := func() *terminal {
				if scenario.mosh {
					return startMoshTerminal(t, s, "attach", session.ID)
				}
				return startTerminal(t, s, binary, "attach", session.ID)
			}
			p := attach()
			p.waitFor("\x1b[?1006h")
			write(t, filepath.Join(s.home, "ready"), "", 0600)
			p.waitFor("READY")
			p.waitFor("52;c;")
			up, down := "\x1b[<64;10;10M", "\x1b[<65;10;10M"
			p.send(up)
			if mouse {
				p.send(down)
			} else {
				p.send("\x02s\x1b[5~q")
			}
			paste := "\x1b[200~pasted 界\nsecond line\x1b[201~"
			p.send(paste)
			p.until(func() bool {
				b, _ := os.ReadFile(filepath.Join(s.home, "received"))
				return strings.Contains(string(b), paste)
			})
			got := read(t, filepath.Join(s.home, "received"))
			if mouse && (!strings.Contains(got, "\x1b[<64;") || !strings.Contains(got, "\x1b[<65;")) {
				t.Fatal("wheel events lost", got)
			}
			if err := pty.Setsize(p.file, &pty.Winsize{Rows: 24, Cols: 50}); err != nil {
				t.Fatal(err)
			}
			zellij := filepath.Join(s.home, ".local/bin/zellij")
			p.until(func() bool {
				out, err := s.run(zellij, "--session", session.ZellijSession, "action", "list-panes", "--json")
				var panes []struct {
					Columns int `json:"pane_columns"`
				}
				return err == nil && json.Unmarshal([]byte(out), &panes) == nil && len(panes) > 0 && panes[0].Columns == 50
			})
			p.send("\x02d")
			p.exit()
			p2 := attach()
			p2.waitFor("READY")
			p2.send("\x02d")
			p2.exit()
			requireContains(t, s.must(binary, "logs", session.ID), "READY")
			s.must(binary, "kill", session.ID)
		})
	}
}
