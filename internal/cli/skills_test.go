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

func TestSkillsBootstrap(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX bootstrap")
	}
	for _, tc := range []struct {
		name, platform, manager, answer, optIn       string
		installed, node, fail, missingResult, modern bool
	}{
		{name: "passthrough", platform: "Linux", manager: "npm", installed: true, node: true},
		{name: "exit status", platform: "Darwin", manager: "pnpm", installed: true, node: true, fail: true},
		{name: "npm install confirmed", platform: "Linux", manager: "npm", node: true, answer: "yes\n"},
		{name: "pnpm install opt in", platform: "Darwin", manager: "pnpm", node: true, optIn: "1"},
		{name: "declined", platform: "Linux", manager: "npm", node: true, answer: "no\n"},
		{name: "eof", platform: "Darwin", manager: "npm", node: true},
		{name: "bootstrap linux", platform: "Linux", optIn: "true"},
		{name: "bootstrap mac", platform: "Darwin", optIn: "yes"},
		{name: "pnpm 12 linux", platform: "Linux", optIn: "1", modern: true},
		{name: "pnpm 12 mac", platform: "Darwin", optIn: "1", modern: true},
		{name: "old node npm", platform: "Linux", manager: "npm", optIn: "1"},
		{name: "failed install", platform: "Linux", manager: "npm", node: true, optIn: "1", fail: true},
		{name: "missing executable", platform: "Linux", manager: "npm", node: true, optIn: "1", missingResult: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			systemTools := map[string]string{}
			for _, name := range []string{"bash", "mktemp", "rm"} {
				path, err := exec.LookPath(name)
				if err != nil {
					t.Fatal(err)
				}
				systemTools[name] = path
			}
			home := t.TempDir()
			bin := filepath.Join(home, "bin")
			if err := os.MkdirAll(bin, 0755); err != nil {
				t.Fatal(err)
			}
			write := func(path, content string) {
				t.Helper()
				if err := os.WriteFile(path, []byte("#!/bin/bash\n"+content), 0755); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("HOME", home)
			t.Setenv("PATH", bin)
			t.Setenv("PNPM_HOME", "")
			t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
			t.Setenv("ASHLEY_INSTALL_SKILLS", tc.optIn)
			t.Setenv("SHELL", "/bin/bash")
			for _, name := range []string{"bash", "mktemp", "rm"} {
				if err := os.Symlink(systemTools[name], filepath.Join(bin, name)); err != nil {
					t.Fatal(err)
				}
			}
			write(filepath.Join(bin, "uname"), "echo "+tc.platform)
			skills := filepath.Join(home, "skills-template")
			write(skills, `printf '<%s>\n' "$@"
printf 'stdin:'
/bin/cat
exit ${SKILLS_TEST_EXIT:-0}`)
			t.Setenv("SKILLS_TEST_EXIT", "0")
			if tc.fail {
				t.Setenv("SKILLS_TEST_EXIT", "7")
			}
			if tc.node {
				write(filepath.Join(bin, "node"), "exit 0")
			}
			t.Setenv("SKILLS_TEST_BIN_SUFFIX", "")
			if tc.modern {
				t.Setenv("SKILLS_TEST_BIN_SUFFIX", "/bin")
			}
			managerBody := `case "$1" in
bin) echo "$PNPM_HOME" ;;
prefix) echo "$HOME/.ashley/tools" ;;
env) /bin/mkdir -p "$PNPM_HOME"; printf '#!/bin/bash\nexit 0\n' > "$PNPM_HOME/node"; /bin/chmod +x "$PNPM_HOME/node" ;;
add|install)
    echo installation >> "$HOME/installs"
    if [[ "${SKILLS_TEST_EXIT:-0}" != 0 ]]; then exit "$SKILLS_TEST_EXIT"; fi
    if [[ "${SKILLS_TEST_NO_RESULT:-}" == 1 ]]; then exit 0; fi
    target="$PNPM_HOME${SKILLS_TEST_BIN_SUFFIX}"
    if [[ "$1" == install ]]; then target="$HOME/.ashley/tools/bin"; fi
    /bin/mkdir -p "$target"
    /bin/cp "$HOME/skills-template" "$target/skills"
    ;;
esac`
			t.Setenv("SKILLS_TEST_NO_RESULT", "")
			if tc.missingResult {
				t.Setenv("SKILLS_TEST_NO_RESULT", "1")
			}
			write(filepath.Join(home, "pnpm-template"), managerBody)
			if tc.manager != "" {
				write(filepath.Join(bin, tc.manager), managerBody)
			}
			if tc.installed {
				write(filepath.Join(bin, "skills"), strings.TrimPrefix(mustReadSkills(t, skills), "#!/bin/bash\n"))
			}
			write(filepath.Join(bin, "curl"), `while [[ "$1" != -o ]]; do shift; done
/bin/cat > "$2" <<'INSTALL'
/bin/mkdir -p "$PNPM_HOME${SKILLS_TEST_BIN_SUFFIX}"
/bin/cp "$HOME/pnpm-template" "$PNPM_HOME${SKILLS_TEST_BIN_SUFFIX}/pnpm"
INSTALL`)
			var out, stderr bytes.Buffer
			err := skillsCommand([]string{"add", "owner/repo with spaces", "--yes", "--help", "$(touch bad)", ""}, strings.NewReader(tc.answer), &out, &stderr)
			declined := !tc.installed && tc.optIn == "" && tc.answer != "yes\n"
			if tc.fail || declined || tc.missingResult {
				if err == nil {
					t.Fatalf("expected failure: %s", stderr.String())
				}
				if tc.fail {
					var exit execution.ExitError
					if !errors.As(err, &exit) || exit.Code != 7 {
						t.Fatalf("exit: %v", err)
					}
				}
			} else {
				if err != nil {
					t.Fatalf("%v: %s", err, stderr.String())
				}
				if !strings.Contains(out.String(), "<add>\n<owner/repo with spaces>\n<--yes>\n<--help>\n<$(touch bad)>\n<>\nstdin:") {
					t.Fatalf("arguments: %s", out.String())
				}
			}
			_, installErr := os.Stat(filepath.Join(home, "installs"))
			if (declined || tc.installed) && installErr == nil {
				t.Fatal("unexpected installation")
			}
			if !tc.installed && !strings.Contains(stderr.String(), "installation plan") {
				t.Fatal("missing plan")
			}
		})
	}
}

func mustReadSkills(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
