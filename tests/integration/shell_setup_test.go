//go:build integration && (darwin || linux)

package integration

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func shellSetupSandbox(t *testing.T) *sandbox {
	t.Helper()
	s := newSandbox(t)
	delete(s.env, "ASHLEY_DEFER_SHELL_SETUP")
	delete(s.env, "ZDOTDIR")
	s.env["SHELL"] = "/bin/zsh" // The active Bash parent must take precedence.
	return s
}

func startShellSetupTerminal(t *testing.T, s *sandbox, args ...string) *terminal {
	t.Helper()
	// Keep the interactive shell as a parent, including after the final command.
	shellArgs := append([]string{"--noprofile", "--norc", "-i", "-c", `"$@"; result=$?; exit "$result"`, "shell-setup-test"}, args...)
	return startTerminal(t, s, "/bin/bash", shellArgs...)
}

func TestInstallOffersShellSettingsLast(t *testing.T) {
	for _, accept := range []bool{true, false} {
		name := "decline"
		if accept {
			name = "accept"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := shellSetupSandbox(t)
			original := "# Keep existing settings\nexport EDITOR=vi\n"
			file := filepath.Join(s.home, ".bashrc")
			write(t, file, original, 0600)
			p := startShellSetupTerminal(t, s, binary, "install", "--skills-only", "--codex")
			p.waitFor("(recommended) [Y/n]")
			if strings.Contains(p.text(), "Append missing Ashley shell settings?") {
				t.Fatal("shell setup preceded the community choice")
			}
			p.send("no\n")
			p.waitFor("Append missing Ashley shell settings? [y/N]")
			requireContains(t, p.text(), file)
			answer := "no\n"
			if accept {
				answer = "yes\n"
			}
			p.send(answer)
			p.exit()
			data := read(t, file)
			if !strings.HasPrefix(data, original) || (data != original) != accept {
				t.Fatal("shell consent or original settings were not preserved", data)
			}
			if exists(filepath.Join(s.home, ".zshrc")) {
				t.Fatal("updated the login shell instead of the current shell")
			}
			if accept {
				requireContains(t, data, filepath.Join(s.home, ".local", "bin"))
				repeat := startShellSetupTerminal(t, s, binary, "install", "--skills-only", "--codex")
				repeat.waitFor("(recommended) [Y/n]")
				repeat.send("no\n")
				repeat.exit()
				if read(t, file) != data || strings.Contains(repeat.text(), "Append missing Ashley shell settings?") {
					t.Fatal("repeated installation duplicated settings")
				}
			}
		})
	}
}

func TestBinaryInstallersConfigureCustomShellPath(t *testing.T) {
	for _, script := range []string{"scripts/remote-install.sh", "scripts/install.sh", "scripts/dev/install-local.sh"} {
		t.Run(script, func(t *testing.T) {
			t.Parallel()
			s := shellSetupSandbox(t)
			installerEnv(t, s, releaseArchive(t))
			destination := filepath.Join(s.home, "custom tools", "bin")
			args := []string{"/bin/bash", filepath.Join(root, script), "--version", version, "--install-dir", destination}
			if script == "scripts/remote-install.sh" {
				args = append(args, "--binary-only")
			} else if script == "scripts/dev/install-local.sh" {
				args = []string{"/bin/bash", filepath.Join(root, script), binary, destination}
			}
			p := startShellSetupTerminal(t, s, args...)
			// Download verification and the installed launcher each probe terminal colors.
			p.untilTimeout(25*time.Second, func() bool {
				return strings.Contains(p.text(), "Append missing Ashley shell settings? [y/N]")
			})
			p.send("yes\n")
			p.exit()
			data := read(t, filepath.Join(s.home, ".bashrc"))
			requireContains(t, data, destination)
			requireContains(t, data, filepath.Join(s.home, ".local", "bin"))
			if exists(filepath.Join(s.home, ".zshrc")) {
				t.Fatal("installer wrapper selected the wrong shell")
			}
		})
	}
}

func TestShellSetupWithoutTerminalDoesNotWrite(t *testing.T) {
	s := shellSetupSandbox(t)
	output := s.must(binary, "__setup-shell", filepath.Join(s.home, "bin"))
	if output != "" || exists(filepath.Join(s.home, ".bashrc")) || exists(filepath.Join(s.home, ".zshrc")) {
		t.Fatal("noninteractive setup changed shell settings", output)
	}
}
