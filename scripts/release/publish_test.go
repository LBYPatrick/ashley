//go:build darwin || linux

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublisher(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "version")
	if out, err := exec.Command("go", "build", "-o", helper, ".").CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	for _, tc := range []struct {
		name   string
		env    []string
		wantOK bool
	}{{"publish", nil, true}, {"gate-failure", []string{"FAIL_GATE=1"}, false}, {"wrong-branch", []string{"TEST_BRANCH=feature"}, false}, {"stray-files", []string{"STRAY= M unrelated.go"}, false}, {"incomplete-stable", []string{"V=1.0.0"}, false}, {"missing-notes", []string{"NOTES=/nonexistent-notes"}, false}, {"preview", []string{"YES="}, true}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			put := func(name, data string, mode os.FileMode) {
				t.Helper()
				path := filepath.Join(dir, name)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(data), mode); err != nil {
					t.Fatal(err)
				}
			}
			script, err := os.ReadFile("publish.sh")
			if err != nil {
				t.Fatal(err)
			}
			put("scripts/release/publish.sh", string(script), 0755)
			put("VERSION", "0.4.0\n", 0644)
			put("README.md", badge("0.4.0"), 0644)
			put("docs/migration/status.json", `{"feature":false}`, 0644)
			notes := filepath.Join(dir, "notes with spaces.md")
			put("notes with spaces.md", "# Notes\n\nLiteral `code` and $HOME.\n", 0644)
			put("tools/runner", `#!/bin/bash
set -euo pipefail
name="${0##*/}"
printf '%s' "$name" >> "$COMMAND_LOG"
printf '\t%s' "$@" >> "$COMMAND_LOG"
printf '\n' >> "$COMMAND_LOG"
case "$name" in
 git)
  case "$1" in
   branch) echo "${TEST_BRANCH:-main}";;
   status) echo "${STRAY:-}";;
   rev-list) echo 0;;
   show-ref) exit 1;;
   rev-parse) echo abc123;;
   diff) exit 1;;
  esac;;
 gh) if [[ "$1" == repo ]]; then echo https://github.com/example/ashley; fi;;
 go) [[ "$1" == run && "$2" == ./scripts/release ]]; shift 2; exec "$VERSION_HELPER" "$@";;
 make) exit "${FAIL_GATE:-0}";;
esac
`, 0755)
			for _, name := range []string{"git", "gh", "go", "make"} {
				if err := os.Symlink("runner", filepath.Join(dir, "tools", name)); err != nil {
					t.Fatal(err)
				}
			}
			log := filepath.Join(dir, "commands.log")
			cmd := exec.Command("/bin/bash", filepath.Join(dir, "scripts/release/publish.sh"))
			cmd.Env = append(os.Environ(), "PATH="+filepath.Join(dir, "tools")+":/usr/bin:/bin", "V=0.4.0-alpha.1", "YES=1", "NOTES="+notes, "COMMAND_LOG="+log, "VERSION_HELPER="+helper)
			cmd.Env = append(cmd.Env, tc.env...)
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.wantOK {
				t.Fatalf("%v\n%s", err, out)
			}
			data, _ := os.ReadFile(log)
			commands := string(data)
			if !tc.wantOK || tc.name == "preview" {
				if strings.Contains(commands, "git\tpush") || strings.Contains(commands, "gh\trelease\tcreate") {
					t.Fatal("failure/preview published", commands)
				}
				if tc.name == "preview" && (!strings.Contains(commands, "make\tgate") || strings.Contains(commands, "git\tcommit")) {
					t.Fatal(commands)
				}
				return
			}
			gate := strings.Index(commands, "make\tgate")
			push := strings.Index(commands, "git\tpush\torigin\tmain")
			tag := strings.Index(commands, "git\tpush\torigin\tv0.4.0-alpha.1")
			if gate < 0 || push <= gate || tag <= push {
				t.Fatal("incorrect publish ordering", commands)
			}
			var create string
			for _, line := range strings.Split(commands, "\n") {
				if strings.HasPrefix(line, "gh\trelease\tcreate") {
					create = line
				}
			}
			for _, flag := range []string{"--draft", "--prerelease", "--verify-tag", "--notes-file\t" + notes} {
				if !strings.Contains(create, flag) {
					t.Fatal("missing release argument", flag, create)
				}
			}
			actual, _ := os.ReadFile(notes)
			if string(actual) != "# Notes\n\nLiteral `code` and $HOME.\n" {
				t.Fatal("notes altered")
			}
		})
	}
}
