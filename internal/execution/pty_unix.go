//go:build !windows

package execution

import (
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/creack/pty"
	"github.com/muesli/cancelreader"
)

// runLogged preserves a real terminal for agents while recording every output
// byte locally, independently of SSH/Mosh attachment and pane scrollback.
func runLogged(cmd *exec.Cmd, path string, stdin io.Reader, stdout io.Writer) error {
	log, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer log.Close()
	size := &pty.Winsize{Rows: 24, Cols: 80}
	input, ok := stdin.(*os.File)
	if ok {
		if s, e := pty.GetsizeFull(input); e == nil {
			size = s
		}
	}
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	terminal, err := pty.StartWithSize(cmd, size)
	if err != nil {
		return err
	}
	defer terminal.Close()
	if ok && term.IsTerminal(input.Fd()) {
		state, e := term.MakeRaw(input.Fd())
		if e == nil {
			defer term.Restore(input.Fd(), state)
		}
	}
	reader, err := cancelreader.NewReader(stdin)
	if err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		return err
	}
	defer reader.Close()
	defer reader.Cancel()
	go io.Copy(terminal, reader)
	resized := make(chan os.Signal, 1)
	signal.Notify(resized, syscall.SIGWINCH)
	defer signal.Stop(resized)
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case <-resized:
				if ok {
					pty.InheritSize(input, terminal)
				}
			case <-done:
				return
			}
		}
	}()
	outputDone := make(chan struct{})
	go func() { io.Copy(io.MultiWriter(stdout, log), terminal); close(outputDone) }()
	err = cmd.Wait()
	reader.Cancel()
	select {
	case <-outputDone:
	case <-time.After(2 * time.Second):
		terminal.Close()
		<-outputDone
	}
	return err
}
