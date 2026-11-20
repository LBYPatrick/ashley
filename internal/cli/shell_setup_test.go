package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LBYPatrick/ashley/internal/shellsetup"
)

func TestShellSetupConsent(t *testing.T) {
	for _, tc := range []struct {
		name, answer string
		accept       bool
	}{{"yes", "yes\n", true}, {"short yes", "Y\n", true}, {"no", "n\n", false}, {"default", "\n", false}, {"EOF", "", false}} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			file := filepath.Join(home, ".zshrc")
			original := "# Existing settings\nexport EDITOR=vi\n"
			if err := os.WriteFile(file, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			cfg := shellsetup.Config{Home: home, Shell: "zsh", Paths: []string{filepath.Join(home, ".local", "bin")}}
			var out bytes.Buffer
			if err := setupShellWithInput(cfg, strings.NewReader(tc.answer), &out); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(data), original) || (string(data) != original) != tc.accept {
				t.Fatalf("unexpected shell settings: %q", data)
			}
			if !strings.Contains(out.String(), file) || !strings.Contains(out.String(), "Append missing Ashley shell settings? [y/N]") {
				t.Fatalf("missing review or question: %s", out.String())
			}
			if tc.accept {
				out.Reset()
				if err := setupShellWithInput(cfg, strings.NewReader("yes\n"), &out); err != nil {
					t.Fatal(err)
				}
				if out.Len() != 0 {
					t.Fatalf("repeated setup asked again: %s", out.String())
				}
			}
		})
	}
}

func TestShellSetupSkippedDuringUnattendedOperations(t *testing.T) {
	for _, name := range []string{"ASHLEY_AUTOMATED", "ASHLEY_BACKGROUND_SYNC", "ASHLEY_UPDATE_REFRESH", "ASHLEY_DEFER_SHELL_SETUP"} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("SHELL", "/bin/zsh")
			t.Setenv(name, "1")
			var out bytes.Buffer
			if err := setupShell("", nil, &out); err != nil {
				t.Fatal(err)
			}
			if entries, err := os.ReadDir(home); err != nil || len(entries) != 0 || out.Len() != 0 {
				t.Fatalf("unattended setup changed shell state: %v %v %q", entries, err, out.String())
			}
		})
	}
}

func TestShellSetupUnsupportedShellAndCommand(t *testing.T) {
	home := t.TempDir()
	var out bytes.Buffer
	err := setupShellWithInput(shellsetup.Config{Home: home, Shell: "tcsh", Paths: []string{filepath.Join(home, "bin")}}, strings.NewReader("yes\n"), &out)
	if err != nil || !strings.Contains(out.String(), "Shell setup is unavailable") {
		t.Fatalf("unsupported shell: %v %s", err, out.String())
	}
	if entries, err := os.ReadDir(home); err != nil || len(entries) != 0 {
		t.Fatalf("unsupported shell changed files: %v %v", entries, err)
	}
	for _, args := range [][]string{{"__setup-shell"}, {"__setup-shell", ""}, {"__setup-shell", "a", "b"}} {
		if err := Run(args, &out, &out); err == nil {
			t.Fatalf("invalid shell setup command succeeded: %v", args)
		}
	}
	if err := Run([]string{"__setup-shell", "--check"}, &out, &out); err != nil {
		t.Fatal(err)
	}
}

func TestShellSetupUsesAvailableCommandsBeforePNPMDirectories(t *testing.T) {
	for _, tc := range []struct {
		name            string
		available       []string
		managed         []string
		wantDirectories []string
		wantPNPM        bool
	}{
		{name: "Homebrew pnpm already works", available: []string{"pnpm", "node", "skills"}, managed: []string{"pnpm", "bin/pnpm", "bin/node", "bin/skills"}},
		{name: "Corepack shim already works", available: []string{"pnpm", "node"}},
		{name: "stale empty pnpm directories"},
		{name: "legacy pnpm needs PATH", managed: []string{"pnpm"}, wantDirectories: []string{"."}, wantPNPM: true},
		{name: "current pnpm needs PATH", managed: []string{"bin/pnpm"}, wantDirectories: []string{"bin"}, wantPNPM: true},
		{name: "prefer current layout", managed: []string{"pnpm", "bin/pnpm"}, wantDirectories: []string{"bin"}, wantPNPM: true},
		{name: "working pnpm with missing managed node", available: []string{"pnpm"}, managed: []string{"bin/node"}, wantDirectories: []string{"bin"}},
		{name: "working pnpm with missing managed skills", available: []string{"pnpm", "node"}, managed: []string{"bin/skills"}, wantDirectories: []string{"bin"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			bin := filepath.Join(home, "homebrew", "bin")
			pnpm := filepath.Join(home, ".local", "share", "pnpm")
			if err := os.MkdirAll(filepath.Join(pnpm, "bin"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(bin, 0755); err != nil {
				t.Fatal(err)
			}
			writeExecutable := func(path string) {
				t.Helper()
				if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
					t.Fatal(err)
				}
			}
			for _, name := range tc.available {
				writeExecutable(filepath.Join(bin, name))
			}
			for _, name := range tc.managed {
				writeExecutable(filepath.Join(pnpm, filepath.FromSlash(name)))
			}
			t.Setenv("HOME", home)
			t.Setenv("PATH", bin)
			t.Setenv("PNPM_HOME", pnpm)
			cfg := shellsetup.Config{Home: home, Shell: "zsh"}
			addShellToolPaths(&cfg, nil)
			plan, err := shellsetup.Prepare(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if (cfg.PNPMHome != "") != tc.wantPNPM || strings.Count(plan.Content, "export PATH=") != len(tc.wantDirectories) {
				t.Fatalf("unexpected pnpm settings: home=%q, content=%s", cfg.PNPMHome, plan.Content)
			}
			for _, directory := range tc.wantDirectories {
				if !strings.Contains(plan.Content, "export PATH='"+filepath.Join(pnpm, directory)+"'") {
					t.Fatalf("missing required directory %q: %s", directory, plan.Content)
				}
			}
			if len(tc.wantDirectories) == 0 && plan.Content != "" {
				t.Fatalf("working commands caused an unnecessary question: %s", plan.Content)
			}
		})
	}
}

func TestShellSetupIgnoresUnusableToolsAndKeepsHomebrew(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "custom-homebrew", "bin")
	pnpm := filepath.Join(home, "pnpm")
	tools := filepath.Join(home, ".ashley", "tools", "bin")
	for _, directory := range []string{bin, pnpm, tools} {
		if err := os.MkdirAll(directory, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte("fixture"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pnpm, "pnpm"), []byte("not executable"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(home, "missing-skills"), filepath.Join(tools, "skills")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("PATH", bin)
	t.Setenv("PNPM_HOME", pnpm)
	cfg := shellsetup.Config{Home: home, Shell: "zsh"}
	addShellToolPaths(&cfg, []string{"codex"})
	if len(cfg.Paths) != 0 || cfg.PNPMHome != "" {
		t.Fatalf("proposed settings for available or unusable tools: %+v", cfg)
	}
	if err := os.Remove(filepath.Join(tools, "skills")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tools, "skills"), []byte("fixture"), 0755); err != nil {
		t.Fatal(err)
	}
	addShellToolPaths(&cfg, []string{"codex"})
	if len(cfg.Paths) != 1 || cfg.Paths[0] != tools {
		t.Fatalf("missing actual managed skills directory: %+v", cfg)
	}
}
