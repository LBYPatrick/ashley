//go:build integration && (darwin || linux)

package integration

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

var root, _ = filepath.Abs("../..")
var binary = filepath.Join(root, "build", "ash-go")
var version = strings.TrimSpace(readVersion())

func readVersion() string { b, _ := os.ReadFile(filepath.Join(root, "VERSION")); return string(b) }

type sandbox struct {
	t    *testing.T
	home string
	env  map[string]string
}

func newSandbox(t *testing.T) *sandbox {
	t.Helper()
	s := &sandbox{t: t, home: t.TempDir(), env: map[string]string{}}
	canonical, err := filepath.EvalSymlinks(s.home)
	if err != nil {
		t.Fatal(err)
	}
	s.home = canonical
	for _, pair := range os.Environ() {
		k, v, _ := strings.Cut(pair, "=")
		s.env[k] = v
	}
	for _, key := range []string{"CLAUDE_CONFIG_DIR", "CODEX_HOME", "GROK_HOME", "GROK_BIN_DIR", "OPENCODE_CONFIG_DIR", "ASHLEY_DIR", "ASHLEY_VERSION", "ASHLEY_INSTALL_DIR", "ASHLEY_REPO", "TMUX", "NO_COLOR", "GOOS", "GOARCH"} {
		delete(s.env, key)
	}
	s.env["HOME"] = s.home
	s.env["PATH"] = "/usr/bin:/bin"
	s.env["XDG_DATA_HOME"] = filepath.Join(s.home, "data")
	s.env["XDG_CONFIG_HOME"] = filepath.Join(s.home, ".config")
	return s
}
func (s *sandbox) environment() []string {
	var result []string
	for k, v := range s.env {
		result = append(result, k+"="+v)
	}
	return result
}
func (s *sandbox) command(ctx context.Context, name string, args ...string) *exec.Cmd {
	c := exec.CommandContext(ctx, name, args...)
	c.Dir = s.home
	c.Env = s.environment()
	return c
}
func (s *sandbox) run(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	out, err := s.command(ctx, name, args...).CombinedOutput()
	return string(out), err
}
func (s *sandbox) must(name string, args ...string) string {
	s.t.Helper()
	out, err := s.run(name, args...)
	if err != nil {
		s.t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return out
}
func write(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}
func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func copyFile(t *testing.T, from, to string) {
	t.Helper()
	b := read(t, from)
	info, err := os.Stat(from)
	if err != nil {
		t.Fatal(err)
	}
	write(t, to, b, info.Mode().Perm())
}
func exists(path string) bool { _, err := os.Stat(path); return err == nil }
func requireContains(t *testing.T, text, part string) {
	t.Helper()
	if !strings.Contains(text, part) {
		t.Fatalf("missing %q in %s", part, text)
	}
}
func link(t *testing.T, target, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
}
func isLink(path string) bool {
	info, e := os.Lstat(path)
	return e == nil && info.Mode()&os.ModeSymlink != 0
}
func historyPath(s *sandbox) string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(s.home, "Library/Application Support/ashley/history.db")
	}
	return filepath.Join(s.env["XDG_DATA_HOME"], "ashley/history.db")
}
func legacyDB(t *testing.T, s *sandbox) {
	t.Helper()
	path := historyPath(s)
	os.MkdirAll(filepath.Dir(path), 0755)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE invocations (id INTEGER PRIMARY KEY AUTOINCREMENT,timestamp TEXT NOT NULL,skill TEXT NOT NULL,question TEXT NOT NULL DEFAULT '',cwd TEXT NOT NULL DEFAULT '',permission TEXT NOT NULL DEFAULT 'default',detached INTEGER NOT NULL DEFAULT 0,session_id TEXT NOT NULL DEFAULT ''); INSERT INTO invocations(timestamp,skill,question) VALUES ('2024-01-01T00:00:00+00:00','feat','Old session retained');`)
	if err != nil {
		t.Fatal(err)
	}
}
func releaseArchive(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	name := fmt.Sprintf("ashley-%s-%s-%s.tar.gz", version, runtime.GOOS, runtime.GOARCH)
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, item := range []struct{ name, path string }{{"ash", binary}, {"LICENSE", filepath.Join(root, "LICENSE")}} {
		data := read(t, item.path)
		if err := tw.WriteHeader(&tar.Header{Name: item.name, Mode: 0755, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(tw, data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	write(t, path+".sha256", fmt.Sprintf("%x  %s\n", sha256.Sum256([]byte(read(t, path))), name), 0644)
	return path
}
func installerEnv(t *testing.T, s *sandbox, archive string) {
	t.Helper()
	tools := filepath.Join(s.home, "tools")
	write(t, filepath.Join(tools, "curl"), `#!/bin/bash
set -euo pipefail
url=""; output=""
while [[ $# -gt 0 ]]; do
 case "$1" in
 -o) output="$2"; shift 2;;
 --retry) shift 2;;
 -fsSL) shift;;
 https://github.com/*/releases/download/*|https://raw.githubusercontent.com/*/main/scripts/install.sh) url="$1"; shift;;
 *) exit 90;;
 esac
done
if [[ "$url" == https://raw.githubusercontent.com/* ]]; then cp "$INSTALLER_FIXTURE" "$output"; else cp "$FIXTURE_DIR/${url##*/}" "$output"; fi
`, 0755)
	s.env["PATH"] = tools + ":/usr/bin:/bin"
	s.env["FIXTURE_DIR"] = filepath.Dir(archive)
	s.env["INSTALLER_FIXTURE"] = filepath.Join(root, "scripts/install.sh")
}
func checkSkills(t *testing.T, s *sandbox, dirs []string) {
	t.Helper()
	for _, dir := range dirs {
		path := filepath.Join(s.home, dir, "skills/a-feat/SKILL.md")
		if len(read(t, path)) < 1000 {
			t.Fatal("empty skill", path)
		}
		resolved, e := filepath.EvalSymlinks(path)
		if e != nil {
			t.Fatal(e)
		}
		rel, e := filepath.Rel(filepath.Join(s.home, ".ashley/generated"), resolved)
		if e != nil || !filepath.IsLocal(rel) {
			t.Fatal("skill still depends on external source", path)
		}
	}
}

var agentDirs = []string{".claude", ".codex", ".grok", ".config/opencode", ".kilo"}
