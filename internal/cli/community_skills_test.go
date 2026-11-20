package cli

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/LBYPatrick/ashley/internal/execution"
)

func TestInstallCommunitySkills(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX bootstrap")
	}
	for _, tc := range []struct {
		name, answer, optIn, failRepo string
		cfg                           *automatedInstall
		installed, wantCLI, wantErr   bool
		wantCalls                     int
	}{
		{name: "default accepts missing CLI", answer: "\n", wantCLI: true, wantCalls: 3},
		{name: "yes accepts missing CLI", answer: "yes\n", wantCLI: true, wantCalls: 3},
		{name: "short yes", answer: "Y\n", installed: true, wantCalls: 3},
		{name: "no declines missing CLI", answer: "no\n"},
		{name: "no declines installed CLI", answer: "n\n", installed: true},
		{name: "unknown answer declines", answer: "later\n"},
		{name: "EOF declines"},
		{name: "explicit CLI opt in survives decline", answer: "no\n", optIn: "1", wantCLI: true},
		{name: "automated bundle with CLI", cfg: &automatedInstall{CommunitySkills: true}, installed: true, wantCalls: 3},
		{name: "automated bundle and dependencies", cfg: &automatedInstall{CommunitySkills: true, InstallSkills: true}, wantCLI: true, wantCalls: 3},
		{name: "automated dependency opt out", cfg: &automatedInstall{CommunitySkills: true}, optIn: "1", wantErr: true},
		{name: "automated none overrides environment", cfg: &automatedInstall{}, optIn: "1"},
		{name: "automated CLI only", cfg: &automatedInstall{InstallSkills: true}, wantCLI: true},
		{name: "failed bundle stops remaining commands", answer: "yes\n", installed: true, failRepo: "GoogleChrome/modern-web-guidance", wantCalls: 2, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bash, err := exec.LookPath("bash")
			if err != nil {
				t.Fatal(err)
			}
			home := t.TempDir()
			bin := filepath.Join(home, "bin")
			if err := os.MkdirAll(bin, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(bash, filepath.Join(bin, "bash")); err != nil {
				t.Fatal(err)
			}
			write := func(path, body string) {
				t.Helper()
				if err := os.WriteFile(path, []byte("#!/bin/bash\n"+body), 0755); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("HOME", home)
			t.Setenv("PATH", bin)
			t.Setenv("PNPM_HOME", filepath.Join(home, "pnpm"))
			t.Setenv("ASHLEY_AUTOMATED", "")
			t.Setenv("ASHLEY_INSTALL_SKILLS", tc.optIn)
			t.Setenv("SKILLS_TEST_LOG", filepath.Join(home, "community-calls"))
			t.Setenv("SKILLS_TEST_FAIL_REPO", tc.failRepo)
			write(filepath.Join(bin, "uname"), "echo Linux\n")
			write(filepath.Join(bin, "node"), "exit 0\n")
			write(filepath.Join(bin, "npm"), `case "$1" in
prefix) echo "$HOME/.ashley/tools" ;;
install)
    echo installed >> "$HOME/cli-installs"
    /bin/mkdir -p "$HOME/.ashley/tools/bin"
    /bin/cp "$HOME/skills-template" "$HOME/.ashley/tools/bin/skills"
    ;;
esac
`)
			skills := `printf '%s\n' "$*" >> "$SKILLS_TEST_LOG"
if [[ "$2" == "$SKILLS_TEST_FAIL_REPO" ]]; then exit 7; fi
name="${2##*/}"
dir="$HOME/.agents/skills/$name"
/bin/mkdir -p "$dir"
printf '# %s\n' "$name" > "$dir/SKILL.md"
`
			write(filepath.Join(home, "skills-template"), skills)
			if tc.installed {
				write(filepath.Join(bin, "skills"), skills)
			}
			var out, stderr bytes.Buffer
			err = installCommunitySkillsWithInput([]string{"claude", "codex"}, tc.cfg, strings.NewReader(tc.answer), &out, &stderr)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error=%v, stderr=%s", err, stderr.String())
			}
			if tc.failRepo != "" {
				var exit execution.ExitError
				if !errors.As(err, &exit) || exit.Code != 7 {
					t.Fatalf("exit status: %v", err)
				}
			}
			prompt := "Install all emilkowalski/skills, GoogleChrome/modern-web-guidance, and find-skills for claude, codex? (recommended) [Y/n] "
			if tc.cfg == nil && !strings.HasPrefix(out.String(), prompt) {
				t.Fatalf("prompt: %q", out.String())
			}
			if tc.cfg != nil && strings.Contains(out.String(), "[Y/n]") {
				t.Fatalf("automated installation prompted: %s", out.String())
			}
			_, err = os.Stat(filepath.Join(home, "cli-installs"))
			if (err == nil) != tc.wantCLI {
				t.Fatalf("CLI installation: %v", err)
			}
			data, err := os.ReadFile(filepath.Join(home, "community-calls"))
			if tc.wantCalls == 0 {
				if !os.IsNotExist(err) {
					t.Fatalf("unexpected community installation: %s, error=%v", data, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var want strings.Builder
			for _, args := range communitySkillCommands([]string{"claude", "codex"})[:tc.wantCalls] {
				want.WriteString(strings.Join(args, " ") + "\n")
			}
			if string(data) != want.String() {
				t.Fatalf("commands: %s", data)
			}
		})
	}
}
