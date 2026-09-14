package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

// tuiCommand retains native CLI output while Bubble Tea has released the terminal.
func tuiCommand(args []string, root string, stdout, stderr io.Writer) error {
	if len(args) == 0 || (args[0] != "skills" && args[0] != "install") {
		return fmt.Errorf("unsupported TUI command")
	}
	var err error
	if args[0] == "install" {
		err = runSkills(nil, os.Stdin, stdout, stderr, []string{"ASHLEY_SKILLS_ENSURE=1"})
	}
	if err == nil {
		if root != "" {
			args = append([]string{"--root", root}, args...)
		}
		err = Run(args, stdout, stderr)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
	}
	fmt.Fprint(stdout, "\nPress Enter to return to Ashley…")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	return err
}
