package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/LBYPatrick/ashley/internal/agents"
	"github.com/LBYPatrick/ashley/internal/config"
	"github.com/LBYPatrick/ashley/internal/execution"
	"github.com/LBYPatrick/ashley/internal/history"
	"github.com/LBYPatrick/ashley/internal/hooks"
	"github.com/LBYPatrick/ashley/internal/invocation"
	"github.com/LBYPatrick/ashley/internal/sessions"
	"github.com/LBYPatrick/ashley/internal/skills"
)

func runOptions(args []string) (invocation.Options, bool, error) {
	var o invocation.Options
	detached := false
	var keys, positionals []string
	literal := false
	for _, arg := range args {
		if literal {
			positionals = append(positionals, arg)
			continue
		}
		switch arg {
		case "--":
			literal = true
		case "-c", "--claude":
			keys = append(keys, "claude")
		case "-o", "--codex":
			keys = append(keys, "codex")
		case "--grok", "--opencode", "--kilo":
			keys = append(keys, strings.TrimPrefix(arg, "--"))
		case "-dsp", "--dangerously-skip-permissions":
			o.DSP = true
		case "--auto":
			o.Auto = true
		case "--normal":
			o.Normal = true
		case "-afk", "--afk", "--away-from-keyboard", "--leon":
			o.AFK = true
		case "--detached":
			detached = true
		default:
			if strings.HasPrefix(arg, "-") {
				return o, false, fmt.Errorf("unknown run option: %s (use -- before literal question text)", arg)
			}
			positionals = append(positionals, arg)
		}
	}
	key, err := agents.Select(keys...)
	if err != nil {
		return o, false, err
	}
	if o.Normal && (o.DSP || o.Auto || o.AFK) {
		return o, false, fmt.Errorf("--normal cannot be combined with another permission mode")
	}
	o.Agent = key
	if len(positionals) == 0 {
		return o, false, fmt.Errorf("a skill or pipeline is required")
	}
	o.Skill = positionals[0]
	o.Question = strings.Join(positionals[1:], " ")
	return o, detached, nil
}
func runCommand(command string, args []string, catalog skills.Catalog, stdout, stderr io.Writer) error {
	o, detached, err := runOptions(args)
	if err != nil {
		return err
	}
	prefs, err := config.User()
	if err != nil {
		return err
	}
	cfg := prefs.Load()
	if o.Agent == "" {
		o.Agent = prefs.LoadAgent()
	}
	if !o.Normal && !o.DSP && !o.Auto && !o.AFK {
		switch cfg.PermissionMode {
		case "auto":
			o.Auto = true
		case "dsp":
			o.DSP = true
		case "afk":
			o.AFK = true
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dbPath := history.Path(home, runtime.GOOS, os.Getenv("XDG_DATA_HOME"))
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	builder := invocation.Builder{Catalog: catalog, Home: home}
	if command == "pipe" {
		if detached {
			return fmt.Errorf("pipelines do not support --detached")
		}
		steps := cfg.ResolvePipeline(o.Skill)
		if len(steps) == 0 {
			return fmt.Errorf("empty pipeline")
		}
		p := present(stdout)
		p.heading("Pipeline")
		p.field("Steps", strings.Join(steps, " → "))
		p.field("Agent", agents.Get(o.Agent).Label)
		for i, skill := range steps {
			step := o
			step.Skill = skill
			if i > 0 {
				step.Question = ""
			}
			p.section(fmt.Sprintf("%d / %d · %s", i+1, len(steps), skill))
			job, err := prepareJob(ctx, builder, step, cfg, cwd, dbPath, "", false, stdout, stderr)
			if err != nil {
				return err
			}
			if err := execution.Run(ctx, job, os.Stdin, stdout, stderr); err != nil {
				return err
			}
		}
		p.success("Pipeline complete.")
		return nil
	}
	if err := ensureTmux(stdout, stderr); err != nil {
		return err
	}
	manager, err := sessions.User()
	if err != nil {
		return err
	}
	session, err := manager.Prepare(sessions.Session{Skill: o.Skill, Question: o.Question, CWD: cwd, Agent: o.Agent})
	if err != nil {
		return err
	}
	job, err := prepareJob(ctx, builder, o, cfg, cwd, dbPath, session.ID, detached, stdout, stderr)
	if err != nil {
		return err
	}
	session.PermissionMode = job.Context.Permission
	if err := os.MkdirAll(manager.Dir, 0700); err != nil {
		return err
	}
	path, err := execution.SaveJob(manager.Dir, job)
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		os.Remove(path)
		return err
	}
	session, err = manager.Start(session, []string{executable, "__execute", path})
	if err != nil {
		os.Remove(path)
		store, e := history.Open(dbPath)
		if e == nil {
			store.RecordOutcome(job.InvocationID, 1, 0)
			store.Close()
		}
		return err
	}
	p := present(stdout)
	p.heading("Session started")
	p.field("Session", session.ID)
	p.field("Agent", agents.Get(o.Agent).Label)
	p.field("Log", session.LogFile)
	p.field("Attach", "ash attach "+session.ID)
	if detached {
		return nil
	}
	return attachSession(manager, session, stdout, stderr)
}
func prepareJob(ctx context.Context, builder invocation.Builder, o invocation.Options, cfg config.Config, cwd, dbPath, sessionID string, detached bool, stdout, stderr io.Writer) (execution.Job, error) {
	v, err := builder.Build(o)
	if err != nil {
		return execution.Job{}, err
	}
	hookContext := hooks.Context{CWD: cwd, Skill: o.Skill, Question: o.Question, Permission: v.Permission, SessionID: sessionID}
	h := cfg.HooksFor(o.Skill)
	runner := hooks.Runner{Stdin: os.Stdin, Stdout: stdout, Stderr: stderr}
	if err := runner.Before(ctx, h, hookContext); err != nil {
		return execution.Job{}, fmt.Errorf("before_run hook failed: %w", err)
	}
	store, err := history.Open(dbPath)
	if err != nil {
		return execution.Job{}, err
	}
	defer store.Close()
	id, err := store.Record(history.Invocation{Skill: o.Skill, Question: o.Question, CWD: cwd, Permission: v.Permission, Detached: detached, SessionID: sessionID, AgentType: o.Agent})
	if err != nil {
		return execution.Job{}, err
	}
	return execution.Job{Args: v.Args, Context: hookContext, Hooks: h, HistoryPath: dbPath, InvocationID: id}, nil
}
func ensureTmux(stdout, stderr io.Writer) error {
	if _, err := exec.LookPath("tmux"); err == nil {
		return nil
	}
	var commands [][]string
	if runtime.GOOS == "darwin" {
		commands = [][]string{{"brew", "install", "tmux"}}
	} else {
		for _, candidate := range []struct {
			name string
			args [][]string
		}{{"apt-get", [][]string{{"sudo", "apt-get", "update", "-qq"}, {"sudo", "apt-get", "install", "-y", "tmux"}}}, {"dnf", [][]string{{"sudo", "dnf", "install", "-y", "tmux"}}}, {"pacman", [][]string{{"sudo", "pacman", "-S", "--noconfirm", "tmux"}}}} {
			if _, err := exec.LookPath(candidate.name); err == nil {
				commands = candidate.args
				break
			}
		}
	}
	if len(commands) == 0 {
		return fmt.Errorf("tmux is required; install it with your system package manager")
	}
	for _, argv := range commands {
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("install tmux: %w", err)
		}
	}
	_, err := exec.LookPath("tmux")
	return err
}
