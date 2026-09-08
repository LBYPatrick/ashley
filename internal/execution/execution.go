// Package execution runs agents and records completion independently of attachment.
package execution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/LBYPatrick/ashley/internal/config"
	"github.com/LBYPatrick/ashley/internal/history"
	"github.com/LBYPatrick/ashley/internal/hooks"
)

// ExitError preserves the agent's exit status through the Ashley CLI.
type ExitError struct{ Code int }

func (e ExitError) Error() string { return fmt.Sprintf("agent exited with status %d", e.Code) }

// Job is the private serialized context handed to the supervisor inside tmux.
type Job struct {
	Args         []string
	Context      hooks.Context
	Hooks        config.Hooks
	HistoryPath  string
	InvocationID int64
}

// Run executes an agent, records its actual status, and runs completion hooks.
func Run(ctx context.Context, job Job, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(job.Args) == 0 {
		return fmt.Errorf("empty agent command")
	}
	start := time.Now()
	cmd := exec.CommandContext(ctx, job.Args[0], job.Args[1:]...)
	cmd.Dir = job.Context.CWD
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.WaitDelay = 2 * time.Second
	runErr := cmd.Run()
	code := 0
	if runErr != nil {
		code = 1
		var exit *exec.ExitError
		if errors.As(runErr, &exit) {
			code = exit.ExitCode()
			if code < 0 {
				code = 130
			}
		}
		if ctx.Err() != nil {
			code = 130
		}
	}
	store, err := history.Open(job.HistoryPath)
	if err == nil {
		err = store.RecordOutcome(job.InvocationID, code, time.Since(start).Seconds())
		err = errors.Join(err, store.Close())
	}
	if err != nil {
		fmt.Fprintln(stderr, "History update:", err)
	}
	job.Context.ExitCode = &code
	runner := hooks.Runner{Stdin: stdin, Stdout: stdout, Stderr: stderr}
	// Completion hooks get their own bounded context even after agent cancellation.
	if hookErr := runner.After(context.Background(), job.Hooks, job.Context); hookErr != nil {
		fmt.Fprintln(stderr, "Completion hook:", hookErr)
	}
	if code != 0 {
		return ExitError{code}
	}
	return err
}

// SaveJob writes private context without putting large prompts on tmux's command line.
func SaveJob(dir string, job Job) (string, error) {
	data, err := json.Marshal(job)
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, ".ashley-job-*.json")
	if err != nil {
		return "", err
	}
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// LoadJob consumes the supervisor context once. The caller owns the returned argv.
func LoadJob(path string) (Job, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Job{}, err
	}
	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return Job{}, err
	}
	if len(job.Args) == 0 || job.HistoryPath == "" || job.InvocationID <= 0 {
		return Job{}, fmt.Errorf("invalid session job")
	}
	if err := os.Remove(path); err != nil {
		return Job{}, err
	}
	return job, nil
}
