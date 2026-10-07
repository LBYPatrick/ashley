//go:build windows

package execution

import (
	"fmt"
	"io"
	"os/exec"
)

func runLogged(cmd *exec.Cmd, path string, stdin io.Reader, stdout io.Writer) error {
	return fmt.Errorf("persistent agent sessions require a Unix terminal (use WSL on Windows)")
}
