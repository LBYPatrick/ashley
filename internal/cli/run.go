package cli

import (
	"context"
	"fmt"
	"io"
	"os"
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
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if literal {
			positionals = append(positionals, arg)
			continue
		}
		switch arg {
		case "-n", "--name":
			if i+1 == len(args) || strings.HasPrefix(args[i+1], "-") {
				return o, false, fmt.Errorf("%s requires a session name", arg)
			}
			i++
			o.Name = strings.TrimSpace(args[i])
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
			if strings.HasPrefix(arg, "--name=") || strings.HasPrefix(arg, "-n=") {
				_, o.Name, _ = strings.Cut(arg, "=")
				o.Name = strings.TrimSpace(o.Name)
				continue
			}
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
	return launch(command, o, detached, catalog, stdout, stderr)
}

func launch(command string, o invocation.Options, detached bool, catalog skills.Catalog, stdout, stderr io.Writer) error {
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
	if o.WorkDir != "" {
		cwd = o.WorkDir
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
	if _, err := agents.FindBinary(agents.Get(o.Agent).Binary); err != nil {
		return fmt.Errorf("%s not found; install the agent before starting a session", agents.Get(o.Agent).Label)
	}
	if err := ensureZellij(stdout, stderr); err != nil {
		return err
	}
	manager, err := sessions.User()
	if err != nil {
		return err
	}
	session, err := manager.Prepare(sessions.Session{Name: o.Name, Skill: o.Skill, Question: o.Question, CWD: cwd, Agent: o.Agent})
	if err != nil {
		return err
	}
	job, err := prepareJob(ctx, builder, o, cfg, cwd, dbPath, session.ID, detached, stdout, stderr)
	if err != nil {
		return err
	}
	session.PermissionMode = job.Context.Permission
	job.LogFile = session.LogFile
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
	if session.Name != "" {
		p.field("Name", session.Name)
	}
	p.field("Agent", agents.Get(o.Agent).Label)
	p.field("Log", session.LogFile)
	p.field("Attach", "ash attach "+session.ID)
	if detached {
		return nil
	}
	return attachSession(manager, session, stdout, stderr)
}
func prepareJob(ctx context.Context, builder invocation.Builder, o invocation.Options, cfg config.Config, cwd, dbPath, sessionID string, detached bool, stdout, stderr io.Writer) (execution.Job, error) {
	agentSessionID := o.ResumeID
	if agentSessionID == "" && (o.Agent == "claude" || o.Agent == "grok") {
		var err error
		agentSessionID, err = agents.NewConversationID()
		if err != nil {
			return execution.Job{}, err
		}
		o.ExtraFlags = append(append([]string{}, o.ExtraFlags...), "--session-id", agentSessionID)
	}
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
	id, err := store.Record(history.Invocation{Name: o.Name, Skill: o.Skill, Question: o.Question, CWD: cwd, Permission: v.Permission, Detached: detached, SessionID: sessionID, AgentType: o.Agent, AgentSessionID: agentSessionID})
	if err != nil {
		return execution.Job{}, err
	}
	return execution.Job{Args: v.Args, Context: hookContext, Hooks: h, HistoryPath: dbPath, InvocationID: id, Agent: o.Agent, AgentSessionID: agentSessionID}, nil
}
func ensureZellij(stdout, stderr io.Writer) error { return sessions.EnsureZellij(stdout) }
