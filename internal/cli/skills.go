package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"

	ashley "github.com/LBYPatrick/ashley"
	"github.com/LBYPatrick/ashley/internal/execution"
)

func skillsCommand(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	return runSkills(args, stdin, stdout, stderr, nil)
}

func runSkills(args []string, stdin io.Reader, stdout, stderr io.Writer, env []string) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("ash skills requires macOS or Linux; run Ashley inside WSL on Windows")
	}
	script, err := ashley.Assets.ReadFile("scripts/skills.sh")
	if err != nil {
		return err
	}
	// -c leaves stdin available for confirmation and the native skills UI.
	argv := append([]string{"-c", string(script), "ash skills"}, args...)
	cmd := exec.Command("bash", argv...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
	cmd.Env = append(os.Environ(), env...)
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() >= 0 {
			return execution.ExitError{Code: exit.ExitCode()}
		}
		return fmt.Errorf("run skills.sh: %w", err)
	}
	return nil
}
