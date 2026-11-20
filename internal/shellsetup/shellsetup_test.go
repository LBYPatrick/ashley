package shellsetup

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStartupFiles(t *testing.T) {
	for _, tc := range []struct {
		name, shell, existing, want string
		login, custom               bool
	}{
		{name: "zsh", shell: "/bin/zsh", want: ".zshrc"},
		{name: "custom zsh", shell: "zsh", custom: true, want: "zsh/.zshrc"},
		{name: "bash", shell: "bash", want: ".bashrc"},
		{name: "login bash", shell: "bash", login: true, want: ".bash_profile"},
		{name: "existing bash login", shell: "bash", login: true, existing: ".bash_login", want: ".bash_login"},
		{name: "existing bash profile", shell: "bash", login: true, existing: ".profile", want: ".profile"},
		{name: "fish", shell: "fish", want: ".config/fish/config.fish"},
		{name: "custom fish", shell: "/usr/bin/fish", custom: true, want: "config/fish/config.fish"},
		{name: "login sh", shell: "/bin/sh", login: true, want: ".profile"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			cfg := Config{Home: home, Shell: tc.shell, Login: tc.login, Paths: []string{filepath.Join(home, ".local/bin")}}
			if tc.custom {
				cfg.ZDotDir, cfg.XDGConfigHome = filepath.Join(home, "zsh"), filepath.Join(home, "config")
			}
			if tc.existing != "" {
				writeFile(t, filepath.Join(home, tc.existing), "# existing\n", 0600)
			}
			plan, err := Prepare(cfg)
			if err != nil || plan.File != filepath.Join(home, filepath.FromSlash(tc.want)) {
				t.Fatalf("plan=%+v, error=%v", plan, err)
			}
		})
	}
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".profile"), "", 0600)
	writeFile(t, filepath.Join(home, ".bash_login"), "", 0600)
	writeFile(t, filepath.Join(home, ".bash_profile"), "", 0600)
	plan, err := Prepare(Config{Home: home, Shell: "bash", Login: true})
	if err != nil || plan.File != filepath.Join(home, ".bash_profile") {
		t.Fatalf("login preference: %+v %v", plan, err)
	}
}

func TestExistingSettings(t *testing.T) {
	for _, tc := range []struct {
		name, shell, content string
		present, pnpmSet     bool
	}{
		{name: "export with HOME", shell: "bash", content: `export PATH="$HOME/.local/bin:$PATH"`, present: true},
		{name: "export with braces", shell: "zsh", content: `export PATH="${PATH}:${HOME}/.local/bin/"`, present: true},
		{name: "tilde", shell: "bash", content: `PATH=~/.local/bin:$PATH`, present: true},
		{name: "comment", shell: "bash", content: `# export PATH="$HOME/.local/bin:$PATH"`},
		{name: "literal HOME", shell: "bash", content: `export PATH='$HOME/.local/bin:$PATH'`},
		{name: "unrelated variable", shell: "bash", content: `OTHER="$HOME/.local/bin"`},
		{name: "temporary command environment", shell: "bash", content: `PATH="$HOME/.local/bin" some-command`},
		{name: "echo", shell: "bash", content: `echo 'export PATH="$HOME/.local/bin:$PATH"'`},
		{name: "inline comment", shell: "bash", content: `OTHER=ok # PATH="$HOME/.local/bin:$PATH"`},
		{name: "semicolon", shell: "bash", content: `OTHER=ok; export PATH="$HOME/.local/bin:$PATH" # bin`, present: true},
		{name: "zsh array", shell: "zsh", content: "path=(\n \"$HOME/.local/bin\" # bin\n $path\n)", present: true},
		{name: "zsh append array", shell: "zsh", content: `path+=(~/.local/bin)`, present: true},
		{name: "fish add path", shell: "fish", content: `fish_add_path --path --prepend "$HOME/.local/bin"`, present: true},
		{name: "fish set path", shell: "fish", content: `set -gx PATH ~/.local/bin $PATH`, present: true},
		{name: "fish comment", shell: "fish", content: `# fish_add_path "$HOME/.local/bin"`},
		{name: "fish dry run", shell: "fish", content: `fish_add_path --dry-run "$HOME/.local/bin"`},
		{name: "fish query", shell: "fish", content: `set -q PATH "$HOME/.local/bin"`},
		{name: "empty PNPM setting", shell: "bash", content: `PNPM_HOME=`, pnpmSet: true},
		{name: "pnpm reference", shell: "bash", content: "export PNPM_HOME=\"$HOME/.local/bin\"\nexport PATH=\"$PNPM_HOME:$PATH\"", present: true, pnpmSet: true},
		{name: "fish pnpm reference", shell: "fish", content: "set -gx PNPM_HOME \"$HOME/.local/bin\"\nfish_add_path --path $PNPM_HOME", present: true, pnpmSet: true},
		{name: "commented pnpm", shell: "bash", content: `# export PNPM_HOME="$HOME/.local/bin"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			cfg := Config{Home: home, Shell: tc.shell, PNPMHome: filepath.Join(home, "pnpm"), Paths: []string{filepath.Join(home, ".local/bin")}}
			paths, pnpmSet := existingSettings(cfg, tc.shell, tc.content)
			if paths[cfg.Paths[0]] != tc.present || pnpmSet != tc.pnpmSet {
				t.Fatalf("paths=%v, PNPM_HOME=%v", paths, pnpmSet)
			}
		})
	}
}

func TestApplyPreservesAndRechecks(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		t.Run(shell, func(t *testing.T) {
			home := t.TempDir()
			cfg := Config{Home: home, Shell: shell, PNPMHome: filepath.Join(home, "pnpm"), Paths: []string{filepath.Join(home, "a 'quoted' $dir $(touch bad) \\path"), filepath.Join(home, "bin")}}
			plan, err := Prepare(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Content == "" || !strings.Contains(plan.Content, "# Ashley shell environment") {
				t.Fatalf("missing plan: %+v", plan)
			}
			changed, err := plan.Apply()
			if err != nil || !changed {
				t.Fatalf("apply=%v, error=%v", changed, err)
			}
			first := readFile(t, plan.File)
			if changed, err := plan.Apply(); err != nil || changed {
				t.Fatalf("repeated apply=%v, error=%v", changed, err)
			}
			if readFile(t, plan.File) != first {
				t.Fatal("repeated apply changed contents")
			}
			again, err := Prepare(cfg)
			if err != nil || again.Content != "" {
				t.Fatalf("repeat plan=%+v, error=%v", again, err)
			}
		})
	}
	home := t.TempDir()
	file := filepath.Join(home, ".bashrc")
	original := "# user settings without final newline"
	writeFile(t, file, original, 0600)
	cfg := Config{Home: home, Shell: "bash", Paths: []string{filepath.Join(home, "bin")}}
	plan, err := Prepare(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := plan.Apply(); err != nil || !changed {
		t.Fatalf("apply=%v, error=%v", changed, err)
	}
	if !strings.HasPrefix(readFile(t, file), original+"\n# Ashley shell environment\n") {
		t.Fatal("existing contents changed")
	}
	info, err := os.Stat(file)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("permissions: %v %v", info, err)
	}
	// A setting added after Prepare must prevent a second block.
	writeFile(t, file, "export PATH='"+cfg.Paths[0]+"':\"$PATH\"\n", 0600)
	if changed, err := plan.Apply(); err != nil || changed {
		t.Fatalf("recheck=%v, error=%v", changed, err)
	}
}

func TestApplyPreservesSymlink(t *testing.T) {
	home := t.TempDir()
	target := filepath.Join(home, "shared.rc")
	writeFile(t, target, "# shared\n", 0640)
	link := filepath.Join(home, ".zshrc")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	plan, err := Prepare(Config{Home: home, Shell: "zsh", Paths: []string{filepath.Join(home, "bin")}})
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := plan.Apply(); err != nil || !changed {
		t.Fatalf("apply=%v, error=%v", changed, err)
	}
	info, err := os.Lstat(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symbolic link replaced: %v %v", info, err)
	}
	info, err = os.Stat(target)
	if err != nil || info.Mode().Perm() != 0640 || !strings.HasPrefix(readFile(t, target), "# shared\n") {
		t.Fatalf("target changed: %v %v", info, err)
	}
}

func TestGeneratedPOSIXSettings(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is not available")
	}
	home := t.TempDir()
	marker := filepath.Join(home, "should-not-exist")
	path := filepath.Join(home, "space ' $HOME $(touch "+marker+") [*] \\literal")
	tools := filepath.Join(home, "tools")
	plan, err := Prepare(Config{Home: home, Shell: "bash", Paths: []string{path, tools}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bash, "--noprofile", "--norc", "-c", `. "$1"; . "$1"; printf '%s' "$PATH"`, "shell-setup-test", plan.File)
	cmd.Env = []string{"HOME=" + home, "PATH=/usr/bin:/bin"}
	output, err := cmd.CombinedOutput()
	if err != nil || string(output) != path+":"+tools+":/usr/bin:/bin" {
		t.Fatalf("PATH=%q, error=%v", output, err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("shell executed path contents: %v", err)
	}
}

func TestInvalidConfiguration(t *testing.T) {
	home := t.TempDir()
	for _, cfg := range []Config{
		{Home: home, Shell: "unsupported"},
		{Home: home, Shell: "sh"},
	} {
		if _, err := Prepare(cfg); !errors.Is(err, ErrUnsupportedShell) {
			t.Fatalf("unsupported shell: %v", err)
		}
	}
	for _, cfg := range []Config{
		{Home: "relative", Shell: "bash"},
		{Home: home, Shell: "bash", Paths: []string{"relative"}},
		{Home: home, Shell: "bash", Paths: []string{home + "\nexport SECRET=value"}},
		{Home: home, Shell: "bash", PNPMHome: home + "\x00bad"},
		{Home: home, Shell: "zsh", ZDotDir: "relative"},
		{Home: home, Shell: "fish", XDGConfigHome: home + ":invalid"},
	} {
		if _, err := Prepare(cfg); err == nil {
			t.Fatalf("invalid configuration accepted: %+v", cfg)
		}
	}
	if changed, err := (Plan{}).Apply(); err == nil || changed {
		t.Fatalf("unprepared plan: %v %v", changed, err)
	}
}

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
