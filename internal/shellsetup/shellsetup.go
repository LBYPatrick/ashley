// Package shellsetup prepares shell startup settings without executing shell code.
package shellsetup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrUnsupportedShell indicates that Ashley cannot select a startup file.
var ErrUnsupportedShell = errors.New("unsupported shell startup configuration")

// Config specifies the shell startup file and the environment settings to add.
type Config struct {
	Home, Shell                      string
	Login                            bool
	ZDotDir, XDGConfigHome, PNPMHome string
	Paths                            []string
}

// Plan describes settings that are absent from the selected startup file.
type Plan struct {
	File, Shell, Content string
	config               Config
}

// Prepare reads the startup file and prepares settings that are not present.
func Prepare(cfg Config) (Plan, error) {
	var err error
	if cfg.Home, err = directory(cfg.Home); err != nil {
		return Plan{}, fmt.Errorf("shell home: %w", err)
	}
	for _, value := range []*string{&cfg.ZDotDir, &cfg.XDGConfigHome, &cfg.PNPMHome} {
		if *value != "" {
			if *value, err = directory(*value); err != nil {
				return Plan{}, err
			}
		}
	}
	paths := make([]string, 0, len(cfg.Paths))
	seen := make(map[string]bool)
	for _, path := range cfg.Paths {
		path, err = directory(path)
		if err != nil {
			return Plan{}, err
		}
		if !seen[path] {
			paths = append(paths, path)
			seen[path] = true
		}
	}
	cfg.Paths = paths
	file, shell, err := startupFile(cfg)
	if err != nil {
		return Plan{}, err
	}
	data, err := readStartup(file)
	if err != nil {
		return Plan{}, err
	}
	return Plan{File: file, Shell: shell, Content: settings(cfg, shell, data), config: cfg}, nil
}

// Apply rechecks the file and appends settings that remain absent.
// It preserves existing contents, file permissions, and symbolic links.
func (p Plan) Apply() (bool, error) {
	if p.File == "" || p.config.Home == "" {
		return false, fmt.Errorf("shell setup requires a prepared plan")
	}
	data, err := readStartup(p.File)
	if err != nil {
		return false, err
	}
	content := settings(p.config, p.Shell, data)
	if content == "" {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(p.File), 0755); err != nil {
		return false, fmt.Errorf("create shell configuration directory: %w", err)
	}
	file, err := os.OpenFile(p.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return false, fmt.Errorf("open shell startup file: %w", err)
	}
	_, writeErr := file.WriteString(content)
	closeErr := file.Close()
	if writeErr != nil {
		return false, fmt.Errorf("append shell startup settings: %w", writeErr)
	}
	if closeErr != nil {
		return false, fmt.Errorf("close shell startup file: %w", closeErr)
	}
	return true, nil
}

func directory(value string) (string, error) {
	if value == "" || !filepath.IsAbs(value) || strings.ContainsAny(value, "\x00\r\n:") {
		return "", fmt.Errorf("invalid shell directory: %q", value)
	}
	return filepath.Clean(value), nil
}

func startupFile(cfg Config) (string, string, error) {
	shell := filepath.Base(cfg.Shell)
	switch shell {
	case "zsh":
		dir := cfg.ZDotDir
		if dir == "" {
			dir = cfg.Home
		}
		return filepath.Join(dir, ".zshrc"), shell, nil
	case "bash":
		if !cfg.Login {
			return filepath.Join(cfg.Home, ".bashrc"), shell, nil
		}
		for _, name := range []string{".bash_profile", ".bash_login", ".profile"} {
			path := filepath.Join(cfg.Home, name)
			if _, err := os.Stat(path); err == nil {
				return path, shell, nil
			} else if !os.IsNotExist(err) {
				return "", "", fmt.Errorf("inspect shell startup file: %w", err)
			}
		}
		return filepath.Join(cfg.Home, ".bash_profile"), shell, nil
	case "fish":
		dir := cfg.XDGConfigHome
		if dir == "" {
			dir = filepath.Join(cfg.Home, ".config")
		}
		return filepath.Join(dir, "fish", "config.fish"), shell, nil
	case "sh":
		if cfg.Login {
			return filepath.Join(cfg.Home, ".profile"), shell, nil
		}
	}
	return "", "", fmt.Errorf("%w: %s", ErrUnsupportedShell, cfg.Shell)
}

func readStartup(path string) (string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read shell startup file: %w", err)
	}
	return string(data), nil
}

func settings(cfg Config, shell, current string) string {
	paths, pnpmSet := existingSettings(cfg, shell, current)
	var lines []string
	quote := quotePOSIX
	if shell == "fish" {
		quote = quoteFish
	}
	if cfg.PNPMHome != "" && !pnpmSet {
		if shell == "fish" {
			lines = append(lines, "set -gx PNPM_HOME "+quote(cfg.PNPMHome))
		} else {
			lines = append(lines, "export PNPM_HOME="+quote(cfg.PNPMHome))
		}
	}
	// Each command prepends one directory. Add lower-priority directories first.
	for index := len(cfg.Paths) - 1; index >= 0; index-- {
		path := cfg.Paths[index]
		if paths[path] {
			continue
		}
		if shell == "fish" {
			lines = append(lines, "fish_add_path --path --prepend "+quote(path))
		} else {
			lines = append(lines,
				"case \":${PATH-}:\" in",
				"  *:"+quote(path)+":*) ;;",
				"  *)",
				"    export PATH="+quote(path)+"\"${PATH:+:$PATH}\"",
				"    ;;",
				"esac")
		}
	}
	if len(lines) == 0 {
		return ""
	}
	prefix := ""
	if current != "" && !strings.HasSuffix(current, "\n") {
		prefix = "\n"
	}
	return prefix + "# Ashley shell environment\n" + strings.Join(lines, "\n") + "\n"
}

func quotePOSIX(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func quoteFish(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	return "'" + strings.ReplaceAll(value, "'", "\\'") + "'"
}
