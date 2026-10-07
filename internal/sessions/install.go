package sessions

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const zellijVersion = "0.45.1"

var zellijAssets = map[string]struct{ name, sha string }{
	"darwin/arm64": {"zellij-aarch64-apple-darwin.tar.gz", "c029ba4fe1927b79ad9f0cdd59155c4dff80777863c85857d4d09b88b56f9891"},
	"darwin/amd64": {"zellij-x86_64-apple-darwin.tar.gz", "8e8bea22737d1652278c51fc5c26c7c22c9855d0ebb9634a84b8873823093114"},
	"linux/arm64":  {"zellij-aarch64-unknown-linux-musl.tar.gz", "05f0802afadd53f8db9514e7cae53c9ae8432fed1b35b8294aa816ee3044a16b"},
	"linux/amd64":  {"zellij-x86_64-unknown-linux-musl.tar.gz", "40bcc2e03f5d5ae8e054e39f676081fe12ab70871506996ba595834c3718eefc"},
}

// ZellijBinary also finds Ashley's rootless install when PATH has not refreshed.
func ZellijBinary() (string, error) {
	home, err := os.UserHomeDir()
	if err == nil {
		for _, dir := range []string{filepath.Join(home, ".local", "bin"), filepath.Join(home, ".cargo", "bin")} {
			p := filepath.Join(dir, "zellij")
			if info, e := os.Stat(p); e == nil && !info.IsDir() && info.Mode()&0111 != 0 {
				return p, nil
			}
		}
	}
	return exec.LookPath("zellij")
}
func compatibleZellij(path string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, path, "--version").Output()
	if err != nil {
		return false
	}
	var major, minor int
	if _, err = fmt.Sscanf(strings.TrimSpace(string(data)), "zellij %d.%d", &major, &minor); err != nil {
		return false
	}
	return major > 0 || minor >= 45
}

// EnsureZellij detects a compatible installation, otherwise installs a pinned,
// checksum-verified official release without root, brew, cargo, or shell piping.
func EnsureZellij(stdout io.Writer) error {
	if path, err := ZellijBinary(); err == nil && compatibleZellij(path) {
		fmt.Fprintln(stdout, "Zellij ready:", path)
		return ensureZellijConfig(stdout)
	}
	asset, ok := zellijAssets[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		return fmt.Errorf("Zellij sessions require Linux or macOS on arm64/amd64; use WSL on Windows")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".local", "bin")
	if err = os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "Installing Zellij", zellijVersion, "to", dir)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://github.com/zellij-org/zellij/releases/download/v"+zellijVersion+"/"+asset.name, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download Zellij: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download Zellij: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 100<<20))
	if err != nil {
		return err
	}
	binary, err := unpackZellij(data, asset.sha)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".zellij-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(binary); err != nil {
		f.Close()
		return err
	}
	if err = f.Chmod(0755); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if !compatibleZellij(f.Name()) {
		return fmt.Errorf("downloaded Zellij failed version validation")
	}
	if err = os.Rename(f.Name(), filepath.Join(dir, "zellij")); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "Zellij installed.")
	return ensureZellijConfig(stdout)
}

// Match Zellij's config overrides and preserve existing user/system settings.
func zellijConfigPath() (string, error) {
	if path := os.Getenv("ZELLIJ_CONFIG_FILE"); path != "" {
		return path, nil
	}
	if dir := os.Getenv("ZELLIJ_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "config.kdl"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	preferred := filepath.Join(home, ".config", "zellij", "config.kdl")
	candidates := []string{preferred}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		candidates = append(candidates, filepath.Join(xdg, "zellij", "config.kdl"))
	}
	if runtime.GOOS == "darwin" {
		candidates = append(candidates, filepath.Join(home, "Library", "Application Support", "org.Zellij-Contributors.Zellij", "config.kdl"))
	}
	candidates = append(candidates, "/etc/zellij/config.kdl")
	for _, path := range candidates {
		if _, err := os.Lstat(path); err == nil {
			return path, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "zellij", "config.kdl"), nil
	}
	return preferred, nil
}

func ensureZellijConfig(stdout io.Writer) error {
	path, err := zellijConfigPath()
	if err != nil {
		return err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// Embed the layout in a separate, uniquely named file, so an existing layout
	// is never replaced. Publish the config only after its layout is complete.
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	layout, err := os.CreateTemp(filepath.Dir(path), "ashley-layout-*.kdl")
	if err != nil {
		return err
	}
	if _, err = layout.WriteString(zellijLayout); err != nil {
		layout.Close()
		os.Remove(layout.Name())
		return err
	}
	if err = layout.Close(); err != nil {
		os.Remove(layout.Name())
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		os.Remove(layout.Name())
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return err
	}
	_, writeErr := fmt.Fprintf(f, "// Ashley starter configuration. Customize freely.\n%s\ndefault_layout %q\n", zellijConfig, layout.Name())
	closeErr := f.Close()
	if err = errors.Join(writeErr, closeErr); err != nil {
		os.Remove(path)
		os.Remove(layout.Name())
		return err
	}
	fmt.Fprintln(stdout, "Created Zellij config:", path)
	return nil
}

func unpackZellij(data []byte, want string) ([]byte, error) {
	if fmt.Sprintf("%x", sha256.Sum256(data)) != want {
		return nil, fmt.Errorf("Zellij checksum mismatch")
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if strings.TrimPrefix(h.Name, "./") != "zellij" {
			continue
		}
		if h.Typeflag != tar.TypeReg || h.Size <= 0 || h.Size > 150<<20 {
			return nil, fmt.Errorf("invalid Zellij binary in archive")
		}
		b, err := io.ReadAll(io.LimitReader(tr, 150<<20))
		if err != nil {
			return nil, err
		}
		return b, nil
	}
	return nil, fmt.Errorf("Zellij archive contains no binary")
}
