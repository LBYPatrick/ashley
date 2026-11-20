package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/LBYPatrick/ashley/internal/agents"
	"github.com/LBYPatrick/ashley/internal/shellsetup"
	"github.com/mattn/go-isatty"
)

func shellSetupSkipped() bool {
	return envEnabled("ASHLEY_AUTOMATED") || envEnabled("ASHLEY_BACKGROUND_SYNC") ||
		envEnabled("ASHLEY_UPDATE_REFRESH") || envEnabled("ASHLEY_DEFER_SHELL_SETUP")
}

func setupShell(installDir string, keys []string, stdout io.Writer) error {
	if shellSetupSkipped() {
		return nil
	}
	input := os.Stdin
	if !isatty.IsTerminal(input.Fd()) {
		tty, err := os.Open("/dev/tty")
		if err != nil {
			return nil
		}
		defer tty.Close()
		input = tty
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if installDir == "" {
		installDir = os.Getenv("ASHLEY_INSTALL_DIR")
	}
	if installDir == "" {
		installDir = filepath.Join(home, ".local", "bin")
		if executable, err := os.Executable(); err == nil && filepath.Base(executable) == "ash" {
			installDir = filepath.Dir(executable)
		}
	}
	installDir, err = filepath.Abs(installDir)
	if err != nil {
		return err
	}
	shell, login := shellsetup.Detect()
	cfg := shellsetup.Config{
		Home: home, Shell: shell, Login: login,
		ZDotDir: os.Getenv("ZDOTDIR"), XDGConfigHome: os.Getenv("XDG_CONFIG_HOME"),
		Paths: []string{installDir},
	}
	addShellToolPaths(&cfg, keys)
	return setupShellWithInput(cfg, input, stdout)
}

func addShellToolPaths(cfg *shellsetup.Config, keys []string) {
	// A command already on PATH needs no additional directory, including Homebrew.
	for _, key := range keys {
		name := agents.Get(key).Binary
		if _, err := exec.LookPath(name); err == nil {
			continue
		}
		if path, err := agents.FindBinary(name); err == nil {
			cfg.Paths = append(cfg.Paths, filepath.Dir(path))
		}
	}
	for _, tool := range []struct {
		directory string
		names     []string
	}{
		{filepath.Join(cfg.Home, ".local", "bin"), []string{"zellij", "skills", "node", "pnpm"}},
		{filepath.Join(cfg.Home, ".ashley", "tools", "bin"), []string{"skills"}},
		{filepath.Join(cfg.Home, ".grok", "bin"), []string{"grok"}},
		{filepath.Join(cfg.Home, ".opencode", "bin"), []string{"opencode"}},
		{filepath.Join(cfg.Home, ".cargo", "bin"), []string{"zellij"}},
	} {
		for _, name := range tool.names {
			if missingShellCommand(name, tool.directory) {
				cfg.Paths = append(cfg.Paths, tool.directory)
				break
			}
		}
	}
	pnpm := os.Getenv("PNPM_HOME")
	if pnpm == "" {
		pnpm = filepath.Join(cfg.Home, "Library", "pnpm")
		if runtime.GOOS != "darwin" {
			data := os.Getenv("XDG_DATA_HOME")
			if data == "" {
				data = filepath.Join(cfg.Home, ".local", "share")
			}
			pnpm = filepath.Join(data, "pnpm")
		}
	}
	for _, name := range []string{"pnpm", "node", "skills"} {
		for _, directory := range []string{filepath.Join(pnpm, "bin"), pnpm} {
			if missingShellCommand(name, directory) {
				cfg.Paths = append(cfg.Paths, directory)
				if name == "pnpm" {
					cfg.PNPMHome = pnpm
				}
				break
			}
		}
	}
}

func missingShellCommand(name, directory string) bool {
	if _, err := exec.LookPath(name); err == nil {
		return false
	}
	_, err := exec.LookPath(filepath.Join(directory, name))
	return err == nil
}

func setupShellWithInput(cfg shellsetup.Config, input io.Reader, stdout io.Writer) error {
	plan, err := shellsetup.Prepare(cfg)
	if errors.Is(err, shellsetup.ErrUnsupportedShell) {
		fmt.Fprintf(stdout, "Shell setup is unavailable for %q. Add the installation directory to PATH in your shell startup file.\n", cfg.Shell)
		return nil
	}
	if err != nil {
		return fmt.Errorf("check shell settings: %w", err)
	}
	if plan.Content == "" {
		return nil
	}
	fmt.Fprintf(stdout, "\nShell environment\nAppend these missing settings to %s:\n\n%s\n", plan.File, plan.Content)
	fmt.Fprint(stdout, "Append missing Ashley shell settings? [y/N] ")
	answer, err := bufio.NewReader(input).ReadString('\n')
	if err != nil {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		changed, err := plan.Apply()
		if err != nil {
			return fmt.Errorf("append shell settings: %w", err)
		}
		if changed {
			fmt.Fprintf(stdout, "Appended shell settings to %s.\nStart a new shell to use these settings.\n", plan.File)
		}
	}
	return nil
}
