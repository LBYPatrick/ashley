//go:build integration && (darwin || linux)

package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/creack/pty"
)

type terminal struct {
	t       *testing.T
	file    *os.File
	cmd     *exec.Cmd
	mu      sync.Mutex
	output  strings.Builder
	done    chan struct{}
	drained chan struct{}
	err     error
}

func startTerminal(t *testing.T, s *sandbox, command string, args ...string) *terminal {
	t.Helper()
	s.env["TERM"] = "xterm-256color"
	s.env["COLORTERM"] = "truecolor"
	p := &terminal{t: t, cmd: s.command(context.Background(), command, args...), done: make(chan struct{}), drained: make(chan struct{})}
	var err error
	p.file, err = pty.StartWithSize(p.cmd, &pty.Winsize{Rows: 40, Cols: 120})
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		defer close(p.drained)
		buf := make([]byte, 65536)
		for {
			n, err := p.file.Read(buf)
			if n > 0 {
				p.mu.Lock()
				p.output.Write(buf[:n])
				p.mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()
	go func() { p.err = p.cmd.Wait(); close(p.done) }()
	t.Cleanup(func() {
		select {
		case <-p.done:
		default:
			p.cmd.Process.Kill()
			<-p.done
		}
		p.file.Close()
		<-p.drained
	})
	return p
}
func (p *terminal) text() string { p.mu.Lock(); defer p.mu.Unlock(); return p.output.String() }
func (p *terminal) send(keys string) {
	p.t.Helper()
	if _, e := p.file.WriteString(keys); e != nil {
		p.t.Fatal(e)
	}
}
func (p *terminal) until(predicate func() bool) {
	p.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !predicate() {
		select {
		case <-p.done:
			p.t.Fatalf("terminal exited: %v\n%s", p.err, p.text())
		default:
		}
		if time.Now().After(deadline) {
			p.t.Fatal("terminal timeout\n", p.text())
		}
		time.Sleep(20 * time.Millisecond)
	}
}
func (p *terminal) waitFor(text string) {
	p.t.Helper()
	p.until(func() bool { return strings.Contains(p.text(), text) })
}
func (p *terminal) exit() {
	p.t.Helper()
	select {
	case <-p.done:
		if p.err != nil {
			p.t.Fatal(p.err, p.text())
		}
	case <-time.After(10 * time.Second):
		p.t.Fatal("terminal did not exit", p.text())
	}
}
func (p *terminal) quit() {
	p.t.Helper()
	p.send("\x03")
	p.exit()
	p.untilOutputDrained()
	requireContains(p.t, p.text(), "\x1b[?1049l")
}
func (p *terminal) untilOutputDrained() {
	p.t.Helper()
	select {
	case <-p.drained:
	case <-time.After(time.Second): // macOS may retain the master until it is closed.
		p.file.Close()
		<-p.drained
	}
}
func TestInteractiveScreens(t *testing.T) {
	for _, mono := range []bool{false, true} {
		for _, tc := range []struct {
			name string
			args []string
		}{{"hub", nil}, {"vibe", []string{"vibe"}}, {"sessions", []string{"sessions"}}, {"history", []string{"history", "browse"}}, {"create", []string{"create"}}} {
			mode := "color"
			if mono {
				mode = "mono"
			}
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				s := newSandbox(t)
				if mono {
					s.env["NO_COLOR"] = "1"
				}
				write(t, filepath.Join(s.home, ".ashley/theme.json"), `{"mode":"dark","preset":"blue"}`, 0644)
				p := startTerminal(t, s, binary, tc.args...)
				p.waitFor("Ashley")
				if tc.name == "hub" {
					before := len(p.text())
					p.send(strings.Repeat("\x1b[B", 6) + "\r")
					p.waitFor("Accent color")
					if len(p.text())-before > 100000 {
						t.Fatal("settings render loop")
					}
					offset := len(p.text())
					p.send("\x1b")
					p.until(func() bool { return strings.Contains(p.text()[offset:], "What would you like") })
				}
				if tc.name == "create" {
					p.send("pty-skill\tTerminal skill\t\tCreated through the terminal\x0e\x0e\x0e\x13")
					p.until(func() bool { return exists(filepath.Join(s.home, ".ashley/skills/pty-skill.jsonc")) })
					requireContains(t, s.must(binary, "prompt", "pty-skill"), "Created through the terminal")
				}
				p.quit()
			})
		}
	}
}

func TestComposerWorkspaceJourney(t *testing.T) {
	s := newSandbox(t)
	p := startTerminal(t, s, binary)
	p.waitFor("What would you like") // First launch goes directly to Home.
	p.send("\x1b[50;5u")             // Ctrl+2
	p.waitFor("Start a new conversation")
	p.send("\x0e") // Ctrl+N: name the draft.
	p.waitFor("Session name")
	p.send("Login repair\r")
	p.waitFor("Name  Login repair")
	p.send("Fix login\rKeep sessions intact")
	p.waitFor("Keep sessions intact")
	p.send("\x13") // Ctrl+S: skill chooser.
	p.waitFor("Search skills")
	p.send("/")
	p.waitFor("Results")
	p.send("debug\r\r")
	p.until(func() bool { return strings.Contains(p.text(), "Skill  a-debug") })
	offset := len(p.text())
	p.send("\x1b[27;5;52~") // Ctrl+4: Library.
	p.until(func() bool { return strings.Contains(ansi.Strip(p.text()[offset:]), "Ashley  /  Library") })
	offset = len(p.text())
	p.send("\x1b[50;5u")
	p.until(func() bool { return strings.Contains(p.text()[offset:], "Keep sessions intact") })
	p.send("\t\t\x1b[C")
	p.until(func() bool { return strings.Contains(ansi.Strip(p.text()), "Full access") })
	p.quit()
}
func TestFirstInstallAgentSelection(t *testing.T) {
	for _, tc := range []struct {
		answer string
		dirs   []string
	}{{"6\n", agentDirs}, {"2\n", []string{".codex"}}} {
		t.Run(strings.TrimSpace(tc.answer), func(t *testing.T) {
			s := newSandbox(t)
			p := startTerminal(t, s, binary, "install", "--skills-only")
			p.waitFor("Choice [")
			p.send(tc.answer)
			p.exit()
			checkSkills(t, s, tc.dirs)
			if tc.answer == "2\n" {
				requireContains(t, read(t, filepath.Join(s.home, ".ashley/prefs.json")), "codex")
				if exists(filepath.Join(s.home, ".claude/skills")) {
					t.Fatal("installed unselected agent")
				}
			}
		})
	}
}
func TestTUISyncUsesOnlyBinary(t *testing.T) {
	s := newSandbox(t)
	s.env["PATH"] = ""
	standalone := filepath.Join(s.home, "ash")
	copyFile(t, binary, standalone)
	write(t, filepath.Join(s.home, ".ashley/theme.json"), `{"mode":"dark","preset":"blue"}`, 0644)
	for _, agent := range []string{"claude", "codex", "grok", "opencode", "kilo"} {
		write(t, filepath.Join(s.home, ".local/bin", agent), "not an executable format; detection only", 0755)
	}
	p := startTerminal(t, s, standalone)
	p.waitFor("Ashley")
	p.send(strings.Repeat("\x1b[B", 3) + "\r")
	p.waitFor("Skills synced for Claude Code")
	checkSkills(t, s, agentDirs)
	if exists(filepath.Join(s.home, "generated")) || exists(filepath.Join(s.home, ".venv")) {
		t.Fatal("sync created development files")
	}
	if strings.Count(p.text(), "\x1b[?1049h") != 1 || strings.Contains(p.text(), "\x1b[?1049l") {
		t.Fatal("sync left alternate screen")
	}
	p.quit()
}

func TestCommunitySkillsConfirmation(t *testing.T) {
	for _, answer := range []string{"yes", "no"} {
		t.Run(answer, func(t *testing.T) {
			s := newSandbox(t)
			communityFixtures(t, s)
			p := startTerminal(t, s, binary, "install", "--skills-only", "--codex")
			p.waitFor("Install all emilkowalski/skills plus find-skills for codex?")
			p.send(answer + "\n")
			p.exit()
			if exists(filepath.Join(s.home, "community-calls")) != (answer == "yes") {
				t.Fatal("wrong consent result", p.text())
			}
		})
	}
}

func TestSkillsTUIHandoff(t *testing.T) {
	s := newSandbox(t)
	communityFixtures(t, s)
	write(t, filepath.Join(s.home, "fake-bin", "skills"), "#!/bin/bash\necho 'Installed skills fixture'\n", 0755)
	write(t, filepath.Join(s.home, ".ashley/theme.json"), `{"mode":"dark","preset":"blue"}`, 0644)
	p := startTerminal(t, s, binary)
	p.waitFor("What would you like")
	p.send(strings.Repeat("\x1b[B", 7) + "\r")
	p.waitFor("Find skills")
	p.send(strings.Repeat("\x1b[B", 5) + "\r")
	p.waitFor("Installed skills fixture")
	p.waitFor("Press Enter to return to Ashley")
	select {
	case <-p.done:
		t.Fatal("output disappeared before acknowledgement")
	default:
	}
	offset := len(p.text())
	p.send("\n")
	p.until(func() bool { return strings.Contains(p.text()[offset:], "Skill library") })
	offset = len(p.text())
	p.send("\x1b")
	p.until(func() bool { return strings.Contains(p.text()[offset:], "What would you like") })
	p.send("q")
	p.exit()
}

func TestUpdateSkipsCommunitySetup(t *testing.T) {
	s := newSandbox(t)
	s.env["NO_COLOR"] = "1"
	s.must(binary, "install", "--skills-only", "--codex")
	communityFixtures(t, s)
	s.env["ASHLEY_INSTALL_SKILLS"] = "1"
	s.env["ASHLEY_AUTOMATED"] = "1"
	s.env["ASHLEY_AUTOMATED_CONFIG"] = filepath.Join(s.home, "missing-profile.json")
	p := startTerminal(t, s, binary, "update", "--version", version, "--skip-tools")
	// Both the updater and its refresh child probe terminal colors at startup.
	select {
	case <-p.done:
		if p.err != nil {
			t.Fatalf("update failed: %v\n%s", p.err, p.text())
		}
	case <-time.After(30 * time.Second):
		t.Fatal("update waited for input", p.text())
	}
	if strings.Contains(p.text(), "Install all emilkowalski") || strings.Contains(p.text(), "[y/N]") {
		t.Fatal("update prompted for community setup", p.text())
	}
	if exists(filepath.Join(s.home, "community-calls")) {
		t.Fatal("update invoked skills.sh")
	}
	checkSkills(t, s, []string{agentDirs[1]})
}
