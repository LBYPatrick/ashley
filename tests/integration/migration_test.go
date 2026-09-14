//go:build integration && (darwin || linux)

package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func legacyInstall(t *testing.T) (*sandbox, string, string) {
	s := newSandbox(t)
	// Exercise spaces in all launcher, backup and data paths.
	s.home = filepath.Join(s.home, "home with spaces")
	s.env["HOME"] = s.home
	s.env["XDG_DATA_HOME"] = filepath.Join(s.home, "data")
	s.env["XDG_CONFIG_HOME"] = filepath.Join(s.home, ".config")
	source := filepath.Join(s.home, ".ashley/repo")
	dest := filepath.Join(s.home, ".local/bin/ash")
	write(t, filepath.Join(source, "bin/ash"), "#!/bin/bash\nexit 99 # old runtime must never run\n", 0755)
	write(t, filepath.Join(source, "skills/custom.jsonc"), `{"name":"a-custom","description":"Old custom skill","components":["components/custom.md"],"resources":[],"output":"generated/a-custom/SKILL.md"}`, 0644)
	write(t, filepath.Join(source, "components/custom.md"), "Preserve my workflow.", 0644)
	write(t, filepath.Join(source, "generated/a-custom/SKILL.md"), "My edited custom prompt.", 0644)
	write(t, filepath.Join(source, "generated/a-custom/helper.sh"), "#!/bin/sh\necho helper\n", 0755)
	link(t, "../../.ashley/repo/bin/ash", dest)
	link(t, filepath.Join(source, "generated/a-custom"), filepath.Join(s.home, ".codex/skills/a-custom"))
	write(t, filepath.Join(s.home, ".ashley/prefs.json"), `{"agent":"codex"}`, 0644)
	write(t, filepath.Join(s.home, ".ashley/config.yaml"), "custom: retained\n", 0644)
	legacyDB(t, s)
	return s, source, dest
}
func TestMigrationPreservesDataAndWorksWithoutCheckout(t *testing.T) {
	s, source, dest := legacyInstall(t)
	original := read(t, historyPath(s))
	args := []string{filepath.Join(root, "scripts/migrate-python.sh"), "--binary", binary}
	s.must("/bin/bash", args...)
	if isLink(dest) || read(t, dest) != read(t, binary) {
		t.Fatal("launcher not replaced")
	}
	backups, _ := filepath.Glob(filepath.Join(s.home, ".ashley/migrations/python-to-go-*/ash"))
	if len(backups) != 1 || !isLink(backups[0]) {
		t.Fatal("missing launcher backup", backups)
	}
	target, e := os.Readlink(backups[0])
	if e != nil || target != "../../.ashley/repo/bin/ash" {
		t.Fatal(target, e)
	}
	requireContains(t, read(t, filepath.Join(source, "bin/ash")), "exit 99")
	if read(t, historyPath(s)) != original {
		t.Fatal("migration changed history")
	}
	requireContains(t, s.must(dest, "history", "show"), "Old session retained")
	if read(t, filepath.Join(s.home, ".ashley/config.yaml")) != "custom: retained\n" {
		t.Fatal("config changed")
	}
	helper := filepath.Join(s.home, ".codex/skills/a-custom/helper.sh")
	info, e := os.Stat(helper)
	if e != nil || info.Mode().Perm()&0111 == 0 {
		t.Fatal("helper lost permissions", e)
	}
	resolved, e := filepath.EvalSymlinks(helper)
	if e != nil {
		t.Fatal(e)
	}
	rel, e := filepath.Rel(filepath.Join(s.home, ".ashley/generated"), resolved)
	if e != nil || !filepath.IsLocal(rel) {
		t.Fatal("helper still depends on checkout")
	}
	if e := os.RemoveAll(source); e != nil {
		t.Fatal(e)
	}
	requireContains(t, s.must(dest, "prompt", "custom", "task"), "Preserve my workflow")
	requireContains(t, read(t, filepath.Join(filepath.Dir(helper), "SKILL.md")), "Preserve my workflow")
	override := filepath.Join(s.home, ".ashley/components/custom.md")
	write(t, override, "Newer local edit", 0644)
	s.must("/bin/bash", args...)
	if read(t, override) != "Newer local edit" {
		t.Fatal("rerun overwrote override")
	}
}
func TestMigrationDownload(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		name := "verified"
		if corrupt {
			name = "corrupt"
		}
		t.Run(name, func(t *testing.T) {
			s, _, dest := legacyInstall(t)
			archive := releaseArchive(t)
			installerEnv(t, s, archive)
			if corrupt {
				write(t, archive, "bad archive", 0644)
			}
			out, e := s.run("/bin/bash", filepath.Join(root, "scripts/migrate-python.sh"), "--version", version)
			if corrupt {
				if e == nil || !isLink(dest) || exists(filepath.Join(s.home, ".ashley/migrations")) {
					t.Fatal("failed download modified installation", out)
				}
				return
			}
			if e != nil || isLink(dest) {
				t.Fatal(e, out)
			}
		})
	}
}
func TestMigrationRejectsInvalidInputWithoutReplacingLauncher(t *testing.T) {
	for _, kind := range []string{"script", "invalid-skill"} {
		t.Run(kind, func(t *testing.T) {
			s, source, dest := legacyInstall(t)
			candidate := binary
			if kind == "script" {
				candidate = filepath.Join(source, "bin/ash")
			} else {
				write(t, filepath.Join(source, "skills/custom.jsonc"), "invalid JSON", 0644)
			}
			out, e := s.run("/bin/bash", filepath.Join(root, "scripts/migrate-python.sh"), "--binary", candidate)
			if e == nil || !isLink(dest) {
				t.Fatal("invalid migration replaced launcher", out)
			}
			if kind == "script" {
				requireContains(t, out, "native Ashley release binary")
			} else {
				backups, _ := filepath.Glob(filepath.Join(s.home, ".ashley/migrations/python-to-go-*/ash"))
				if len(backups) == 0 {
					t.Fatal("missing recovery backup")
				}
			}
		})
	}
}
func TestMigrationExplicitSourceAndUserOverrides(t *testing.T) {
	s, source, dest := legacyInstall(t)
	custom := filepath.Join(s.home, "my skills checkout")
	if e := os.Rename(source, custom); e != nil {
		t.Fatal(e)
	}
	os.Remove(dest)
	write(t, dest, "old Python entrypoint", 0755)
	agentLink := filepath.Join(s.home, ".codex/skills/a-custom")
	os.Remove(agentLink)
	link(t, filepath.Join(custom, "generated/a-custom"), agentLink)
	override := filepath.Join(s.home, ".ashley/components/custom.md")
	write(t, override, "Existing user override", 0644)
	s.must("/bin/bash", filepath.Join(root, "scripts/migrate-python.sh"), "--binary", binary, "--source", custom)
	if read(t, dest) != read(t, binary) || read(t, override) != "Existing user override" {
		t.Fatal("migration changed user override")
	}
	resolved, e := filepath.EvalSymlinks(agentLink)
	if e != nil {
		t.Fatal(e)
	}
	rel, e := filepath.Rel(filepath.Join(s.home, ".ashley/generated"), resolved)
	if e != nil || !filepath.IsLocal(rel) {
		t.Fatal("old source link retained")
	}
	backups, _ := filepath.Glob(filepath.Join(s.home, ".ashley/migrations/python-to-go-*/ash"))
	if len(backups) != 1 || read(t, backups[0]) != "old Python entrypoint" {
		t.Fatal("missing original launcher backup")
	}
}

func TestRemoteAutomaticallyMigratesPython(t *testing.T) {
	for _, mode := range []string{"symlink", "copied", "go", "binary-only", "corrupt", "protected"} {
		t.Run(mode, func(t *testing.T) {
			s, source, dest := legacyInstall(t)
			copyFile(t, filepath.Join(root, "tests/integration/testdata/python-launcher.sh"), filepath.Join(source, "bin/ash"))
			if mode == "go" {
				copyFile(t, filepath.Join(root, "bin/ash"), filepath.Join(source, "bin/ash"))
			}
			os.Chmod(filepath.Join(source, "bin/ash"), 0755)
			archive := releaseArchive(t)
			installerEnv(t, s, archive)
			shadow := filepath.Join(s.home, "shadow/ash")
			if mode == "copied" {
				copyFile(t, filepath.Join(source, "bin/ash"), shadow)
			} else {
				link(t, source+"/bin/ash", shadow)
			}
			s.env["PATH"] = filepath.Dir(shadow) + ":" + s.env["PATH"]
			originalHistory := read(t, historyPath(s))
			originalPrefs := read(t, filepath.Join(s.home, ".ashley/prefs.json"))
			if mode == "protected" {
				if os.Geteuid() == 0 {
					t.Skip("root can write protected directories")
				}
				if err := os.Chmod(filepath.Dir(shadow), 0555); err != nil {
					t.Fatal(err)
				}
				defer os.Chmod(filepath.Dir(shadow), 0755)
			}
			if mode == "corrupt" {
				write(t, archive, "bad archive", 0644)
			}
			args := []string{"-s", "--", "--version", version, "--skills-only", "--codex"}
			if mode == "binary-only" {
				// The migration visits Claude first but must retain Codex as default.
				if err := os.MkdirAll(filepath.Join(s.home, ".claude/skills"), 0755); err != nil {
					t.Fatal(err)
				}
				args = []string{"-s", "--", "--version", version, "--binary-only"}
			}
			cmd := s.command(context.Background(), "/bin/bash", args...)
			cmd.Stdin = strings.NewReader(read(t, filepath.Join(root, "scripts/remote-install.sh")))
			out, err := cmd.CombinedOutput()
			if mode == "corrupt" || mode == "protected" {
				if err == nil || !isLink(dest) || exists(filepath.Join(s.home, ".ashley/migrations")) {
					t.Fatalf("failed download changed installation: %s", out)
				}
				return
			}
			if err != nil {
				t.Fatalf("remote migration: %v\n%s", err, out)
			}
			if isLink(dest) || read(t, dest) != read(t, binary) {
				t.Fatal("launcher not replaced")
			}
			target, err := os.Readlink(shadow)
			if err != nil || target != dest {
				t.Fatal("PATH launcher not repaired", target, err)
			}
			requireContains(t, s.must(shadow, "--version"), "ashley "+version)
			var beforePrefs, afterPrefs map[string]any
			if err := json.Unmarshal([]byte(originalPrefs), &beforePrefs); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(read(t, filepath.Join(s.home, ".ashley/prefs.json"))), &afterPrefs); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(beforePrefs, afterPrefs) {
				t.Fatal("preferences changed", afterPrefs)
			}
			if read(t, historyPath(s)) != originalHistory {
				t.Fatal("history changed")
			}
			if read(t, filepath.Join(s.home, ".ashley/config.yaml")) != "custom: retained\n" {
				t.Fatal("config changed")
			}
			backups, _ := filepath.Glob(filepath.Join(s.home, ".ashley/migrations/python-to-go-*/path-ash"))
			if len(backups) != 1 {
				t.Fatal("missing PATH backup", backups)
			}
			if err := os.RemoveAll(source); err != nil {
				t.Fatal(err)
			}
			requireContains(t, s.must(dest, "prompt", "custom", "task"), "Preserve my workflow")
			requireContains(t, read(t, filepath.Join(s.home, ".codex/skills/a-custom/SKILL.md")), "Preserve my workflow")
			// A native reinstall must not create another migration backup.
			s.env["PATH"] = filepath.Dir(dest) + ":" + s.env["PATH"]
			s.must("/bin/bash", filepath.Join(root, "scripts/remote-install.sh"), "--version", version, "--binary-only")
			backups, _ = filepath.Glob(filepath.Join(s.home, ".ashley/migrations/python-to-go-*"))
			if len(backups) != 1 {
				t.Fatal("native reinstall migrated again", backups)
			}
		})
	}
}

func TestRemoteLeavesUnrelatedAshOnPATH(t *testing.T) {
	s := newSandbox(t)
	installerEnv(t, s, releaseArchive(t))
	unrelated := filepath.Join(s.home, "system/ash")
	original := "#!/bin/sh\necho unrelated shell\n"
	write(t, unrelated, original, 0755)
	s.env["PATH"] = filepath.Dir(unrelated) + ":" + s.env["PATH"]
	out := s.must("/bin/bash", filepath.Join(root, "scripts/remote-install.sh"), "--version", version, "--binary-only")
	if read(t, unrelated) != original || isLink(unrelated) {
		t.Fatal("replaced unrelated ash")
	}
	requireContains(t, out, "Another executable is first on PATH")
	if exists(filepath.Join(s.home, ".ashley/migrations")) {
		t.Fatal("fresh installation treated as migration")
	}
}

func TestAutomaticMigrationSkillFailureRetainsLauncherAndPreference(t *testing.T) {
	s, source, dest := legacyInstall(t)
	installerEnv(t, s, releaseArchive(t))
	prefs := read(t, filepath.Join(s.home, ".ashley/prefs.json"))
	write(t, filepath.Join(source, "skills/custom.jsonc"), "invalid JSON", 0644)
	out, err := s.run("/bin/bash", filepath.Join(root, "scripts/remote-install.sh"), "--version", version, "--binary-only")
	if err == nil || !isLink(dest) {
		t.Fatal("failed skill migration replaced launcher", out)
	}
	if read(t, filepath.Join(s.home, ".ashley/prefs.json")) != prefs {
		t.Fatal("failed migration changed preference")
	}
	backups, _ := filepath.Glob(filepath.Join(s.home, ".ashley/migrations/python-to-go-*/ash"))
	if len(backups) != 1 || !isLink(backups[0]) {
		t.Fatal("missing recovery backup", backups)
	}
}
