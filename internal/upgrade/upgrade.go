// Package upgrade detects and upgrades coding-agent installations.
package upgrade

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/LBYPatrick/ashley/internal/agents"
)

// Status describes the installed binary and its update mechanism.
type Status struct {
	Agent                          agents.Agent
	Source, Path, Version, Package string
	Cask                           bool
}

// Runner executes an external command, optionally with an installer on stdin.
type Runner func(context.Context, []string, io.Reader) (string, error)

// Manager supplies command lookup, execution and HTTP transport.
type Manager struct {
	LookPath func(string) (string, error)
	Run      Runner
	Client   *http.Client
}

// CommandRunner connects command output to the CLI or TUI.
func CommandRunner(stdout, stderr io.Writer) Runner {
	return func(ctx context.Context, args []string, stdin io.Reader) (string, error) {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Stdin = stdin
		var output bytes.Buffer
		cmd.Stdout = io.MultiWriter(stdout, &output)
		cmd.Stderr = stderr
		err := cmd.Run()
		return output.String(), err
	}
}
func (m Manager) lookup(name string) (string, error) {
	if m.LookPath != nil {
		return m.LookPath(name)
	}
	return agents.FindBinary(name)
}

// BrewPackage recognizes only Cellar/Caskroom paths, never npm script basenames.
func BrewPackage(resolved, prefix string) (string, bool) {
	if prefix == "" {
		return "", false
	}
	relative, err := filepath.Rel(prefix, resolved)
	if err != nil || !filepath.IsLocal(relative) {
		return "", false
	}
	parts := strings.Split(relative, string(filepath.Separator))
	if len(parts) < 4 {
		return "", false
	}
	switch parts[0] {
	case "Cellar":
		return parts[1], false
	case "Caskroom":
		return parts[1], true
	}
	return "", false
}

// Detect performs bounded local version/source detection without downloading.
func (m Manager) Detect(parent context.Context, key string) Status {
	a := agents.Get(key)
	s := Status{Agent: a, Source: "missing"}
	path, err := m.lookup(a.Binary)
	if err != nil {
		return s
	}
	s.Path = path
	s.Source = "native"
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	output, err := m.Run(ctx, []string{path, "--version"}, nil)
	if err == nil {
		s.Version = strings.TrimSpace(strings.SplitN(strings.TrimSpace(output), "\n", 2)[0])
	}
	if _, err := m.lookup("brew"); err == nil {
		prefix, err := m.Run(ctx, []string{"brew", "--prefix"}, nil)
		if err == nil {
			resolved, err := filepath.EvalSymlinks(path)
			if err == nil {
				brewRoot := strings.TrimSpace(prefix)
				if canonical, e := filepath.EvalSymlinks(brewRoot); e == nil {
					brewRoot = canonical
				}
				s.Package, s.Cask = BrewPackage(resolved, brewRoot)
				if s.Package != "" {
					s.Source = "brew"
				}
			}
		}
	}
	return s
}

// NativeInstall runs the vendor installer without installing Ashley source files.
func (m Manager) NativeInstall(ctx context.Context, key string) error {
	if key == "kilo" {
		_, err := m.Run(ctx, []string{"npm", "install", "-g", "@kilocode/cli"}, nil)
		return err
	}
	urls := map[string]string{"claude": "https://claude.ai/install.sh", "codex": "https://chatgpt.com/codex/install.sh", "grok": "https://x.ai/cli/install.sh", "opencode": "https://opencode.ai/install"}
	url, ok := urls[key]
	if !ok {
		return fmt.Errorf("unknown coding agent: %s", key)
	}
	client := m.Client
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("installer download: %s", response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 8<<20+1))
	if err != nil {
		return err
	}
	if len(data) > 8<<20 {
		return fmt.Errorf("installer exceeds size limit")
	}
	shell := "bash"
	if key == "codex" {
		shell = "sh"
	}
	_, err = m.Run(ctx, []string{shell, "-s"}, bytes.NewReader(data))
	return err
}

// Ensure installs a missing agent without reinstalling an existing executable.
func (m Manager) Ensure(ctx context.Context, key string) error {
	if _, err := m.lookup(agents.Get(key).Binary); err == nil {
		return nil
	}
	if err := m.NativeInstall(ctx, key); err != nil {
		return err
	}
	if _, err := m.lookup(agents.Get(key).Binary); err != nil {
		return fmt.Errorf("%s installer finished but the executable was not found: %w", key, err)
	}
	return nil
}

// Upgrade tries Homebrew, the CLI updater, or the vendor bootstrap as appropriate.
func (m Manager) Upgrade(ctx context.Context, key string) error {
	s := m.Detect(ctx, key)
	if s.Source == "brew" {
		args := []string{"brew", "upgrade"}
		if s.Cask {
			args = append(args, "--cask")
		}
		_, err := m.Run(ctx, append(args, s.Package), nil)
		return err
	}
	if s.Source != "missing" && len(s.Agent.SelfUpdateArgs) > 0 {
		if _, err := m.Run(ctx, append([]string{s.Path}, s.Agent.SelfUpdateArgs...), nil); err == nil {
			return nil
		}
	}
	if err := m.NativeInstall(ctx, key); err != nil {
		return err
	}
	if _, err := m.lookup(agents.Get(key).Binary); err != nil {
		return fmt.Errorf("%s installer finished but the executable was not found: %w", key, err)
	}
	return nil
}
