// Package hooks runs user-configured shell commands around agent invocations.
package hooks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/LBYPatrick/ashley/internal/config"
)

// Context is exposed to hooks through ASHLEY_* environment variables.
type Context struct {
	CWD, Skill, Question, Permission, SessionID string
	ExitCode                                    *int
}

// Environment combines inherited variables with invocation-specific values.
func (c Context) Environment(base []string) []string {
	values := map[string]string{}
	for _, entry := range base {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	values["ASHLEY_SKILL"] = c.Skill
	values["ASHLEY_QUESTION"] = c.Question
	permission := c.Permission
	if permission == "" {
		permission = "default"
	}
	values["ASHLEY_PERMISSION"] = permission
	// Remove stale context inherited from a parent Ashley invocation.
	delete(values, "ASHLEY_EXIT_CODE")
	delete(values, "ASHLEY_SESSION_ID")
	if c.ExitCode != nil {
		values["ASHLEY_EXIT_CODE"] = strconv.Itoa(*c.ExitCode)
	}
	if c.SessionID != "" {
		values["ASHLEY_SESSION_ID"] = c.SessionID
	}
	result := make([]string, 0, len(values))
	for key, value := range values {
		result = append(result, key+"="+value)
	}
	return result
}

// Runner controls hook IO and the per-command timeout (default 60 seconds).
type Runner struct {
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	Timeout        time.Duration
}

// Run executes hooks sequentially and stops at the first failure.
func (r Runner) Run(parent context.Context, commands []string, c Context) error {
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	for _, command := range commands {
		ctx, cancel := context.WithTimeout(parent, timeout)
		cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
		cmd.Dir = c.CWD
		cmd.Env = c.Environment(os.Environ())
		cmd.Stdin = r.Stdin
		cmd.Stdout = r.Stdout
		cmd.Stderr = r.Stderr
		configureProcess(cmd)
		cmd.WaitDelay = time.Second
		err := cmd.Run()
		contextErr := ctx.Err()
		cancel()
		if contextErr != nil {
			return fmt.Errorf("hook %q: %w", command, contextErr)
		}
		if err != nil {
			return fmt.Errorf("hook %q: %w", command, err)
		}
	}
	return nil
}

// Before runs the before-run group; a failure should abort the agent invocation.
func (r Runner) Before(ctx context.Context, h config.Hooks, c Context) error {
	return r.Run(ctx, h.BeforeRun, c)
}

// After always attempts on-error hooks when the agent failed, even if an after hook fails.
func (r Runner) After(ctx context.Context, h config.Hooks, c Context) error {
	afterErr := r.Run(ctx, h.AfterRun, c)
	if c.ExitCode != nil && *c.ExitCode != 0 {
		return errors.Join(afterErr, r.Run(ctx, h.OnError, c))
	}
	return afterErr
}
