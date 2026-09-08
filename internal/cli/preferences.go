package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/LBYPatrick/ashley/internal/agents"
	"github.com/LBYPatrick/ashley/internal/config"
	"github.com/LBYPatrick/ashley/internal/upgrade"
)

func preferences(command string, args []string, stdout, stderr io.Writer) error {
	store, err := config.User()
	if err != nil {
		return err
	}
	if command == "config" {
		if len(args) != 0 {
			return fmt.Errorf("config does not accept arguments")
		}
		path, err := store.Init()
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, "Config:", path)
		editor := os.Getenv("EDITOR")
		if editor == "" {
			fmt.Fprintln(stdout, "Edit it at:", path)
			return nil
		}
		// EDITOR is a user-authored command; keep the filename a separate argument.
		cmd := exec.Command("/bin/sh", "-c", "exec "+editor+` "$1"`, "ash-editor", path)
		cmd.Stdin = os.Stdin
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		return cmd.Run()
	}
	if len(args) > 1 {
		return fmt.Errorf("agent accepts one name")
	}
	if len(args) == 1 {
		if err := store.SaveAgent(args[0]); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "Default agent:", agents.Get(args[0]).Label)
		return nil
	}
	a := agents.Get(store.LoadAgent())
	fmt.Fprintf(stdout, "Default agent: %s (%s)\n", a.Label, a.Key)
	status := (upgrade.Manager{Run: upgrade.CommandRunner(io.Discard, io.Discard)}).Detect(context.Background(), a.Key)
	fmt.Fprintf(stdout, "Version: %s (%s)\n", status.Version, status.Source)
	if binary, err := agents.FindBinary(a.Binary); err == nil {
		fmt.Fprintln(stdout, "Executable:", binary)
	} else {
		fmt.Fprintln(stdout, "Executable: not installed")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, "Skills dir:", agents.SkillsDir(a.Key, home, os.Getenv))
	fmt.Fprintln(stdout, "Available:", strings.Join(agents.Keys(), ", "))
	return nil
}
