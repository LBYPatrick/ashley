// Package sessions manages persistent tmux agent sessions.
package sessions

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// Commander executes tmux commands, optionally accepting configuration on stdin.
type Commander func(args []string, input string) (string, error)

// Tmux executes the installed tmux client.
func Tmux(args []string, input string) (string, error) {
	cmd := exec.Command("tmux", args...)
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tmux %s: %w: %s", strings.Join(args, " "), err, out)
	}
	return string(out), nil
}

var tableBinding = regexp.MustCompile(`(?m)^(bind-key\s+(?:-\S+\s+)*-T\s+)\S+`)

// ConfigureScrolling captures wheel events without changing other sessions.
// Copying the existing key table preserves shortcuts and tmux prefix handling.
func ConfigureScrolling(run Commander, name string) error {
	original, err := run([]string{"show-options", "-Av", "-t", name, "key-table"}, "")
	if err != nil {
		return err
	}
	original = strings.TrimSpace(original)
	table := original
	if !strings.HasPrefix(table, "ashley-scroll-") {
		table = "ashley-scroll-" + original
	}
	if table != original {
		bindings, err := run([]string{"list-keys", "-T", original}, "")
		if err != nil {
			return err
		}
		copied := tableBinding.ReplaceAllStringFunc(bindings, func(binding string) string {
			indices := tableBinding.FindStringSubmatch(binding)
			return indices[1] + "'" + strings.ReplaceAll(table, "'", "'\\''") + "'"
		})
		if _, err := run([]string{"source-file", "-"}, copied); err != nil {
			return err
		}
	}
	for _, args := range [][]string{
		{"bind-key", "-T", table, "WheelUpPane", "copy-mode", "-e", "-t", "="},
		{"bind-key", "-T", table, "WheelDownPane", "if-shell", "-F", "#{pane_in_mode}", "send-keys -X -t = -N 5 scroll-down"},
		{"set-option", "-t", name, "key-table", table},
		{"set-option", "-t", name, "mouse", "on"},
	} {
		if _, err := run(args, ""); err != nil {
			return err
		}
	}
	return nil
}
