package hooks

import "os/exec"

// CommandContext supplies process cancellation on Windows. Full shell and tmux
// workflows run under WSL; Unix process-group signals are unavailable here.
func configureProcess(cmd *exec.Cmd) {}
