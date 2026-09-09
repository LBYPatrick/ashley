//go:build integration && (darwin || linux)

package integration

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestStandaloneBinaryWithoutRuntimes(t *testing.T) {
	s := newSandbox(t)
	s.env["PATH"] = ""
	standalone := filepath.Join(s.home, "ash")
	copyFile(t, binary, standalone)
	for _, args := range [][]string{{"--version"}, {"list"}, {"prompt", "feat", "a task"}, {"generate", "--output", filepath.Join(s.home, "output")}, {"install", "--all", "--skills-only"}} {
		s.must(standalone, args...)
	}
	if !exists(filepath.Join(s.home, "output/generated/a-feat/SKILL.md")) || exists(filepath.Join(s.home, ".venv")) {
		t.Fatal("standalone generation failed")
	}
	checkSkills(t, s, agentDirs)
}
func TestVerifiedInstallerAndFailures(t *testing.T) {
	for _, kind := range []string{"valid", "corrupt", "wrong-checksum-name"} {
		t.Run(kind, func(t *testing.T) {
			s := newSandbox(t)
			archive := releaseArchive(t)
			installerEnv(t, s, archive)
			dest := filepath.Join(s.home, "bin")
			write(t, filepath.Join(dest, "ash"), "old launcher", 0755)
			switch kind {
			case "corrupt":
				write(t, archive, "bad archive", 0644)
			case "wrong-checksum-name":
				write(t, archive+".sha256", strings.Repeat("0", 64)+"  unrelated.tar.gz\n", 0644)
			}
			out, err := s.run("/bin/bash", filepath.Join(root, "scripts/install.sh"), "--version", version, "--install-dir", dest)
			if kind != "valid" {
				if err == nil || read(t, filepath.Join(dest, "ash")) != "old launcher" {
					t.Fatal("bad download replaced launcher", out)
				}
				return
			}
			if err != nil {
				t.Fatal(err, out)
			}
			if strings.TrimSpace(s.must(filepath.Join(dest, "ash"), "--version")) != "ashley "+version {
				t.Fatal("wrong installed version")
			}
			entries, _ := os.ReadDir(dest)
			if len(entries) != 1 {
				t.Fatal("installer left temporary files")
			}
		})
	}
}
func TestReleasePackageContents(t *testing.T) {
	cmd := exec.Command("/bin/bash", filepath.Join(root, "scripts/release/package.sh"))
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	path := filepath.Join(root, "dist", fmt.Sprintf("ashley-%s-%s-%s.tar.gz", version, runtime.GOOS, runtime.GOARCH))
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	gz, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var names []string
	for {
		header, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		names = append(names, header.Name)
		if header.Name == "ash" {
			var magic [4]byte
			io.ReadFull(tr, magic[:])
			if strings.HasPrefix(string(magic[:]), "#!") {
				t.Fatal("packaged script")
			}
		}
	}
	sort.Strings(names)
	if strings.Join(names, ",") != "LICENSE,ash" {
		t.Fatal(names)
	}
	want := fmt.Sprintf("%x", sha256.Sum256([]byte(read(t, path))))
	if strings.Fields(read(t, path+".sha256"))[0] != want {
		t.Fatal("bad checksum")
	}
}
func TestLegacySettingsAndHistory(t *testing.T) {
	s := newSandbox(t)
	s.env["PATH"] = ""
	legacyDB(t, s)
	config := filepath.Join(s.home, ".ashley")
	yaml := "# keep comments\nhooks:\n  global:\n    before_run: make test\n"
	write(t, filepath.Join(config, "config.yaml"), yaml, 0644)
	write(t, filepath.Join(config, "prefs.json"), `{"agent":"codex","extension":42}`, 0644)
	write(t, filepath.Join(config, "theme.json"), `{"mode":"light","preset":"ocean"}`, 0644)
	out := s.must(binary, "history", "show", "--json")
	var rows []map[string]any
	if e := json.Unmarshal([]byte(out), &rows); e != nil {
		t.Fatal(e)
	}
	if len(rows) != 1 || rows[0]["agent_type"] != "claude" || rows[0]["outcome"] != "unknown" {
		t.Fatal(rows)
	}
	requireContains(t, s.must(binary, "agent"), "codex")
	s.must(binary, "agent", "grok")
	s.must(binary, "config")
	var prefs map[string]any
	json.Unmarshal([]byte(read(t, filepath.Join(config, "prefs.json"))), &prefs)
	if prefs["agent"] != "grok" || prefs["extension"] != float64(42) || read(t, filepath.Join(config, "config.yaml")) != yaml {
		t.Fatal("preferences changed")
	}
	requireContains(t, read(t, filepath.Join(config, "theme.json")), "ocean")
	s.must(binary, "history", "clear", "--yes")
	db, e := sql.Open("sqlite", historyPath(s))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var count int
	if e := db.QueryRow("SELECT count(*) FROM invocations").Scan(&count); e != nil || count != 0 {
		t.Fatal(count, e)
	}
}
func TestRemoteBootstrap(t *testing.T) {
	for _, tc := range []struct {
		name        string
		flags, dirs []string
	}{{"binary-only", []string{"--binary-only"}, nil}, {"three-agents", []string{"--skills-only", "--grok", "--opencode", "--kilo"}, agentDirs[2:]}, {"two-agent-forms", []string{"--skills-only", "--agent", "codex", "--agent=claude"}, agentDirs[:2]}} {
		t.Run(tc.name, func(t *testing.T) {
			s := newSandbox(t)
			installerEnv(t, s, releaseArchive(t))
			dest := filepath.Join(s.home, "bin")
			args := append([]string{filepath.Join(root, "scripts/remote-install.sh"), "--version", version, "--install-dir", dest}, tc.flags...)
			s.must("/bin/bash", args...)
			if !exists(filepath.Join(dest, "ash")) || exists(filepath.Join(s.home, ".ashley/repo")) {
				t.Fatal("bootstrap needs source")
			}
			checkSkills(t, s, tc.dirs)
		})
	}
}
func TestRemoteRejectsInvalidOptions(t *testing.T) {
	for _, flags := range [][]string{{"--agent=invalid"}, {"--agent"}, {"--install-dir"}, {"--wat"}} {
		s := newSandbox(t)
		s.env["PATH"] = ""
		out, e := s.run("/bin/bash", append([]string{filepath.Join(root, "scripts/remote-install.sh")}, flags...)...)
		if e == nil || strings.Contains(out, "command not found") {
			t.Fatal(flags, e, out)
		}
	}
}
func TestLocalInstallerReplacesOnlyLauncher(t *testing.T) {
	s := newSandbox(t)
	source := filepath.Join(s.home, "source-launcher")
	write(t, source, "original source launcher", 0755)
	dest := filepath.Join(s.home, "bin")
	link(t, source, filepath.Join(dest, "ash"))
	s.must("/bin/bash", filepath.Join(root, "scripts/dev/install-local.sh"), binary, dest)
	if isLink(filepath.Join(dest, "ash")) || read(t, source) != "original source launcher" {
		t.Fatal("source modified")
	}
	requireContains(t, s.must(filepath.Join(dest, "ash"), "--version"), version)
}
func TestDetachedRunAndCompletionHooks(t *testing.T) {
	tmux, e := exec.LookPath("tmux")
	if e != nil {
		t.Skip("tmux not installed")
	}
	s := newSandbox(t)
	socket := fmt.Sprintf("ashley-go-test-%d", time.Now().UnixNano())
	defer exec.Command(tmux, "-L", socket, "kill-server").Run()
	tools := filepath.Join(s.home, "tools")
	quote := func(v string) string { return "'" + strings.ReplaceAll(v, "'", "'\\''") + "'" }
	write(t, filepath.Join(tools, "tmux"), "#!/bin/sh\nexec "+quote(tmux)+" -L "+socket+" -f /dev/null \"$@\"\n", 0700)
	write(t, filepath.Join(tools, "codex"), "#!/bin/sh\nprintf 'agent-start\\n'\nprintf '%s' \"$*\" > \"$HOME/agent-args\"\nexit 7\n", 0700)
	s.env["PATH"] = tools + ":" + os.Getenv("PATH")
	s.env["TMUX"] = ""
	write(t, filepath.Join(s.home, ".ashley/config.yaml"), "hooks:\n  global:\n    before_run: 'echo before > before-hook'\n    after_run: 'echo $ASHLEY_EXIT_CODE:$ASHLEY_SESSION_ID > after-hook'\n    on_error: 'echo error > error-hook'\n", 0644)
	question := "literal $HOME `text` " + strings.Repeat("x", 8000)
	s.must(binary, "run", "--codex", "--detached", "raw", question)
	var sessions []map[string]any
	json.Unmarshal([]byte(s.must(binary, "sessions", "--json")), &sessions)
	if len(sessions) != 1 {
		t.Fatal(sessions)
	}
	id := sessions[0]["id"].(string)
	deadline := time.Now().Add(8 * time.Second)
	for !exists(filepath.Join(s.home, "error-hook")) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if strings.TrimSpace(read(t, filepath.Join(s.home, "after-hook"))) != "7:"+id || read(t, filepath.Join(s.home, "agent-args")) != question {
		t.Fatal("hook/arguments changed")
	}
	if strings.TrimSpace(read(t, filepath.Join(s.home, "before-hook"))) != "before" {
		t.Fatal("before hook missing")
	}
	var rows []map[string]any
	json.Unmarshal([]byte(s.must(binary, "history", "show", "--json")), &rows)
	if rows[0]["exit_code"] != float64(7) || rows[0]["outcome"] != "failure" || rows[0]["session_id"] != id || rows[0]["detached"] != true {
		t.Fatal(rows)
	}
	requireContains(t, s.must(binary, "logs", id), "agent-start")
	s.must(binary, "kill", id)
	requireContains(t, s.must(binary, "sessions", "--json"), "[]")
	if !exists(sessions[0]["log_file"].(string)) {
		t.Fatal("log deleted")
	}
	jobs, _ := filepath.Glob(filepath.Join(s.home, ".ashley/sessions/.ashley-job-*.json"))
	if len(jobs) != 0 {
		t.Fatal("job file leaked")
	}
}
