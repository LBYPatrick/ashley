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
	p := present(stdout)
	if command == "config" {
		p.heading("Configuration")
		if len(args) != 0 {
			return fmt.Errorf("config does not accept arguments")
		}
		path, err := store.Init()
		if err != nil {
			return err
		}
		p.field("Config", path)
		editor := os.Getenv("EDITOR")
		if editor == "" {
			p.line("Set EDITOR to open this file from ash config.")
			return nil
		}
		// EDITOR is a user-authored command; keep the filename a separate argument.
		cmd := exec.Command("/bin/sh", "-c", "exec "+editor+` "$1"`, "ash-editor", path)
		cmd.Stdin = os.Stdin
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		return cmd.Run()
	}
	p.heading("Coding agent")
	if len(args) > 1 {
		return fmt.Errorf("agent accepts one name")
	}
	if len(args) == 1 {
		if err := store.SaveAgent(args[0]); err != nil {
			return err
		}
		p.success("Default agent: " + agents.Get(args[0]).Label)
		return nil
	}
	a := agents.Get(store.LoadAgent())
	p.field("Default", a.Label+" ("+a.Key+")")
	status := (upgrade.Manager{Run: upgrade.CommandRunner(io.Discard, io.Discard)}).Detect(context.Background(), a.Key)
	p.field("Version", status.Version)
	p.field("Source", status.Source)
	if binary, err := agents.FindBinary(a.Binary); err == nil {
		p.field("Executable", binary)
	} else {
		p.field("Executable", "Not installed · ash install --"+a.Key)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	p.field("Skills", agents.SkillsDir(a.Key, home, os.Getenv))
	p.field("Available", strings.Join(agents.Keys(), ", "))
	return nil
}
