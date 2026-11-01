package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
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
	var cmd *exec.Cmd
	if s.Backend == "zellij" {
		binary, err := sessions.ZellijBinary()
		if err != nil {
			return err
		}
		cmd = exec.Command(binary, "attach", s.ZellijSession)
	} else {
		run := m.Run
		if run == nil {
			run = sessions.Tmux
		}
		if err := sessions.ConfigureScrolling(run, s.TmuxSession); err != nil {
			return err
		}
		cmd = exec.Command("tmux", "attach-session", "-t", s.TmuxSession)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	if m.Alive(s) {
		fmt.Fprintf(stdout, "Session %s remains available: ash attach %s\n", s.ID, s.ID)
	} else if err == nil {
		err = m.Remove(s, false)
	}
	return err
}
func sessionCommand(command string, args []string, stdout, stderr io.Writer) error {
	p := present(stdout)
	manager, err := sessions.User()
	if err != nil {
		return err
	}
	if command == "sessions" {
		asJSON, clean := false, false
		mode := "time"
		var filter sessions.Filter
		var list bool
		flags := flag.NewFlagSet("sessions", flag.ContinueOnError)
		flags.SetOutput(stderr)
		flags.BoolVar(&asJSON, "json", false, "Print JSON")
		flags.BoolVar(&list, "list", false, "Print sessions")
		flags.BoolVar(&clean, "clean", false, "Remove exited sessions")
		flags.StringVar(&mode, "sort", "time", "Sort by time or skill")
		flags.StringVar(&filter.Search, "search", "", "Search session text")
		flags.StringVar(&filter.CWD, "cwd", "", "Filter by working directory substring")
		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil
			}
			return err
		}
		if flags.NArg() != 0 {
			return fmt.Errorf("sessions does not accept positional arguments")
		}
		if mode != "time" && mode != "skill" {
			return fmt.Errorf("sort must be time or skill")
		}
		if clean && (strings.TrimSpace(filter.Search) != "" || strings.TrimSpace(filter.CWD) != "") {
			return fmt.Errorf("--clean cannot be combined with --search or --cwd")
		}
		if clean {
			n, err := manager.CleanDead()
			if err != nil {
				return err
			}
			if !asJSON {
				p.success(fmt.Sprintf("Cleaned %d exited sessions.", n))
			}
		}
		all, err := manager.All()
		if err != nil {
			return err
		}
		all = sessions.Sort(sessions.FilterRows(all, filter), mode)
		if asJSON {
			return json.NewEncoder(stdout).Encode(all)
		}
		p.heading("Sessions")
		if len(all) == 0 {
			if strings.TrimSpace(filter.Search) != "" || strings.TrimSpace(filter.CWD) != "" {
				p.line("No sessions match the filters.")
			} else {
				p.line("No sessions. Start one with ash run <skill>.")
			}
			return nil
		}
		var rows [][]string
		for _, s := range all {
			state := "exited"
			if manager.Alive(s) {
				state = "running"
			}
			agent := s.Agent
			if agent == "" {
				agent = "claude"
			}
			rows = append(rows, []string{s.ID, s.Name, s.Skill, agent, state, s.Elapsed(time.Now()), s.CWD})
		}
		p.table([]string{"ID", "Name", "Skill", "Agent", "Status", "Elapsed", "Directory"}, rows)
		p.section("Next")
		p.line("ash attach <id> · Resume   ash logs <id> · View output")
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
		p.success(fmt.Sprintf("Killed %d session(s).", n))
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
		p.success(fmt.Sprintf("Killed session %s (%s)", s.ID, s.Skill))
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
