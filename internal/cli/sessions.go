package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/LBYPatrick/ashley/internal/sessions"
)

func attachSession(m sessions.Manager, s sessions.Session, stdout, stderr io.Writer) error {
	if !m.Alive(s) {
		return fmt.Errorf("session %s is not running; view its log with ash logs %s", s.ID, s.ID)
	}
	if err := sessions.ConfigureScrolling(m.Run, s.TmuxSession); err != nil {
		return err
	}
	cmd := exec.Command("tmux", "attach-session", "-t", s.TmuxSession)
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	if m.Alive(s) {
		fmt.Fprintf(stdout, "Session %s remains available: ash attach %s\n", s.ID, s.ID)
	} else if err == nil {
		err = m.Remove(s, true)
	}
	return err
}
func sessionCommand(command string, args []string, stdout, stderr io.Writer) error {
	manager, err := sessions.User()
	if err != nil {
		return err
	}
	if command == "sessions" {
		asJSON, clean := false, false
		mode := "time"
		for _, arg := range args {
			switch arg {
			case "--json":
				asJSON = true
			case "--list":
			case "--clean":
				clean = true
			case "--sort=skill":
				mode = "skill"
			default:
				return fmt.Errorf("unknown sessions option: %s", arg)
			}
		}
		if clean {
			n, err := manager.CleanDead()
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "Cleaned %d exited sessions.\n", n)
		}
		all, err := manager.All()
		if err != nil {
			return err
		}
		all = sessions.Sort(all, mode)
		if asJSON {
			return json.NewEncoder(stdout).Encode(all)
		}
		if len(all) == 0 {
			fmt.Fprintln(stdout, "No sessions.")
			return nil
		}
		fmt.Fprintf(stdout, "%-10s %-12s %-12s %-8s %s\n", "ID", "Skill", "Agent", "Status", "Elapsed")
		for _, s := range all {
			state := "exited"
			if manager.Alive(s) {
				state = "running"
			}
			agent := s.Agent
			if agent == "" {
				agent = "claude"
			}
			fmt.Fprintf(stdout, "%-10s %-12s %-12s %-8s %s\n", s.ID, s.Skill, agent, state, s.Elapsed(time.Now()))
		}
		return nil
	}
	follow := false
	tail := 0
	var id string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if command == "logs" && (arg == "--follow" || arg == "-f") {
			follow = true
			continue
		}
		if command == "logs" && (arg == "--tail" || arg == "-n") {
			i++
			if i >= len(args) {
				return fmt.Errorf("--tail requires a count")
			}
			tail, err = strconv.Atoi(args[i])
			if err != nil || tail < 0 {
				return fmt.Errorf("tail must be nonnegative")
			}
			continue
		}
		if id != "" || strings.HasPrefix(arg, "-") {
			return fmt.Errorf("%s requires one session ID", command)
		}
		id = arg
	}
	if id == "" {
		return fmt.Errorf("%s requires a session ID", command)
	}
	if command == "kill" && id == "all" {
		n, err := manager.KillAll()
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Killed %d session(s).\n", n)
		return nil
	}
	s, err := manager.Resolve(id)
	if err != nil {
		return err
	}
	switch command {
	case "attach":
		return attachSession(manager, s, stdout, stderr)
	case "kill":
		if err := manager.Kill(s); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Killed session %s (%s)\n", s.ID, s.Skill)
		return nil
	case "logs":
		text, err := sessions.ReadLog(s, tail)
		if err != nil {
			return err
		}
		if strings.TrimSpace(text) == "" {
			text = "(empty log)"
		}
		fmt.Fprint(stdout, text)
		if !strings.HasSuffix(text, "\n") {
			fmt.Fprintln(stdout)
		}
		if !follow {
			return nil
		}
		file, err := os.Open(s.LogFile)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		defer file.Close()
		if _, err := file.Seek(0, io.SeekEnd); err != nil {
			return err
		}
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				if _, err := io.Copy(stdout, file); err != nil {
					return err
				}
			}
		}
	}
	return fmt.Errorf("unknown session command: %s", command)
}
