package selfupdate

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SourceUpdate preserves explicit developer-checkout updates. End-user updates
// never call this path and do not require git, Go, or downloaded source files.
func SourceUpdate(ctx context.Context, root, destination, branch string, stdout, stderr io.Writer) error {
	if branch == "" || strings.HasPrefix(branch, "-") {
		return fmt.Errorf("invalid development branch")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return fmt.Errorf("development update requires an Ashley Go checkout: %w", err)
	}
	status := exec.CommandContext(ctx, "git", "-C", root, "status", "--porcelain")
	output, err := status.Output()
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(output))) != 0 {
		return fmt.Errorf("commit or stash checkout changes before a development update")
	}
	pull := exec.CommandContext(ctx, "git", "-C", root, "pull", "--ff-only", "origin", branch)
	pull.Stdout = stdout
	pull.Stderr = stderr
	if err := pull.Run(); err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		return err
	}
	version, err := Version(strings.TrimSpace(string(data)))
	if err != nil {
		return err
	}
	temporary, err := os.MkdirTemp("", "ashley-build-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	candidate := filepath.Join(temporary, "ash")
	build := exec.CommandContext(ctx, "go", "build", "-trimpath", "-ldflags=-s -w", "-o", candidate, "./cmd/ash")
	build.Dir = root
	build.Stdout = stdout
	build.Stderr = stderr
	// The developer toolchain builds exactly the same pure-Go executable as CI.
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "CGO_ENABLED=") {
			build.Env = append(build.Env, entry)
		}
	}
	build.Env = append(build.Env, "CGO_ENABLED=0")
	if err := build.Run(); err != nil {
		return err
	}
	binary, err := os.ReadFile(candidate)
	if err != nil {
		return err
	}
	return Install(ctx, destination, version, binary)
}
