package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	ashley "github.com/LBYPatrick/ashley"
	"github.com/LBYPatrick/ashley/internal/install"
	"github.com/LBYPatrick/ashley/internal/selfupdate"
	"github.com/LBYPatrick/ashley/internal/skills"
)

func TestBinaryUpdateRefreshesSkillsAndPreservesUserData(t *testing.T) {
	candidate := filepath.Join(t.TempDir(), "ash")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-o", candidate, "./cmd/ash")
	build.Dir = "../.."
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build actual update candidate: %v\n%s", err, output)
	}
	binary, err := os.ReadFile(candidate)
	if err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	compressed := gzip.NewWriter(&archive)
	writer := tar.NewWriter(compressed)
	if err := writer.WriteHeader(&tar.Header{Name: "ash", Mode: 0755, Size: int64(len(binary))}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(binary); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	filename := fmt.Sprintf("ashley-%s-%s-%s.tar.gz", ashley.Version(), runtime.GOOS, runtime.GOARCH)
	checksum := fmt.Sprintf("%x  %s\n", sha256.Sum256(archive.Bytes()), filename)
	var corrupt atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/latest"):
			fmt.Fprintf(w, `{"tag_name":"v%s"}`, ashley.Version())
		case strings.HasSuffix(r.URL.Path, ".sha256"):
			if corrupt.Load() {
				fmt.Fprintf(w, "%s  %s\n", strings.Repeat("0", 64), filename)
			} else {
				fmt.Fprint(w, checksum)
			}
		case strings.HasSuffix(r.URL.Path, ".tar.gz"):
			w.Write(archive.Bytes())
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, key := range []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "GROK_HOME", "OPENCODE_CONFIG_DIR", "XDG_CONFIG_HOME", "SKIP_TOOL"} {
		t.Setenv(key, "")
	}
	catalog := skills.Catalog{Source: ashley.Assets}
	installer := install.Installer{Home: home, Catalog: catalog}
	if _, err := installer.Install([]string{"codex"}); err != nil {
		t.Fatal(err)
	}
	generated := filepath.Join(home, ".ashley", "generated")
	customized := filepath.Join(generated, "a-feat", "SKILL.md")
	if err := os.WriteFile(customized, []byte("user's local prompt"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(generated, "a-debug", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(home, "bin")
	os.MkdirAll(destination, 0700)
	original := []byte("previous installation")
	installed := filepath.Join(destination, "ash")
	os.WriteFile(installed, original, 0700)
	updater := selfupdate.Updater{APIBase: server.URL, DownloadBase: server.URL, Repo: "test/ashley", Platform: runtime.GOOS, Arch: runtime.GOARCH}
	var output bytes.Buffer
	args := []string{"--install-dir", destination, "--skip-tools"}
	if err := updateWithUpdater(append(args, "--check"), "", catalog, updater, &output, &output); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(installed)
	if !bytes.Equal(data, original) {
		t.Fatal("check changed installation")
	}
	corrupt.Store(true)
	if err := updateWithUpdater(args, "", catalog, updater, &output, &output); err == nil {
		t.Fatal("corrupt download accepted")
	}
	data, _ = os.ReadFile(installed)
	if !bytes.Equal(data, original) {
		t.Fatal("failed download replaced installation")
	}
	corrupt.Store(false)
	if err := updateWithUpdater(args, "", catalog, updater, &output, &output); err != nil {
		t.Fatal(err, output.String())
	}
	data, _ = os.ReadFile(installed)
	if !bytes.Equal(data, binary) {
		t.Fatal("candidate not installed")
	}
	data, _ = os.ReadFile(customized)
	if string(data) == "user's local prompt" || len(data) < 100 {
		t.Fatal("local prompt was not regenerated")
	}
	data, err = os.ReadFile(filepath.Join(home, ".codex", "skills", "a-debug", "SKILL.md"))
	if err != nil || !strings.Contains(string(data), "debug") {
		t.Fatal("new executable did not refresh embedded skills", err)
	}
	version, err := exec.Command(installed, "--version").CombinedOutput()
	if err != nil || strings.TrimSpace(string(version)) != "ashley "+ashley.Version() {
		t.Fatal(string(version), err)
	}
	// The default update path also invokes upgrades through the new executable.
	// Keep the fake agent local so this exercises orchestration without vendor IO.
	agent := filepath.Join(destination, "codex")
	if err := os.WriteFile(agent, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"$HOME/agent-updates\"\necho 'codex 1.0.0'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", destination)
	if err := updateWithUpdater([]string{"--install-dir", destination}, "", catalog, updater, &output, &output); err != nil {
		t.Fatal(err, output.String())
	}
	calls, err := os.ReadFile(filepath.Join(home, "agent-updates"))
	if err != nil || !strings.Contains(string(calls), "update\n") {
		t.Fatal("agent upgrade was not invoked", err)
	}
}
