// Package cli implements the Ashley command-line entry point.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	ashley "github.com/LBYPatrick/ashley"
	"github.com/LBYPatrick/ashley/internal/execution"
	"github.com/LBYPatrick/ashley/internal/project"
	"github.com/LBYPatrick/ashley/internal/skills"
	"github.com/LBYPatrick/ashley/internal/tui"
)

const help = `Ashley — coding-agent skill launcher

Usage: ash [--root REPOSITORY] COMMAND [OPTIONS]

Commands:
  -i, --interactive            Open the interactive hub (also the default)
  vibe / create                Open the skill browser or creator
  history browse               Open the history browser
  list                         List available skills
  prompt [--project DIR] SKILL [QUESTION...]
                               Print a copy-pasteable prompt
  generate [--output DIR] [--project DIR]
                               Generate SKILL.md packages
  detect [DIRECTORY]           Print detected project context as JSON
  history [show|stats|info|prune|clear]
                               Browse and manage invocation history
  install [--all|--both|--AGENT] [--skills-only] [--verbose]
                               Install persistent skills and agent CLIs
  uninstall                    Remove Ashley skill links
  update [--check] [--version VERSION] [--skip-tools]
                               Install a verified binary release and refresh skills
  upgrade [AGENT...] [--all] [--check]
                               Detect or upgrade coding-agent CLIs
  run [OPTIONS] SKILL [QUESTION...]
                               Launch an agent in a persistent tmux session
  pipe [OPTIONS] PIPELINE [QUESTION...]
                               Run a named or plus-separated skill pipeline
  sessions [--list|--json]      List persistent sessions
  attach ID / logs ID / kill ID Manage a session (kill all is supported)
  agent [NAME]                 Show or set the preferred coding agent
  config                       Initialize or edit configuration
  version                      Print version

Built-in skills are embedded; --root uses a custom Ashley repository instead.
Run modes: --normal, --auto, -dsp, --afk. --normal overrides the configured default.
`

// Run executes one command without terminating the process.
func Run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("ash-go", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "Custom Ashley repository")
	interactive := flags.Bool("interactive", false, "Open interactive Ashley")
	flags.BoolVar(interactive, "i", false, "Open interactive Ashley")
	version := flags.Bool("version", false, "Print version")
	flags.Usage = func() { fmt.Fprint(stdout, help) }
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	args = flags.Args()
	if *version {
		fmt.Fprintln(stdout, "ashley "+ashley.Version())
		return nil
	}
	var source fs.FS = ashley.Assets
	if *root != "" {
		source = os.DirFS(*root)
	} else if home, err := os.UserHomeDir(); err == nil {
		source = skills.Overlay{User: os.DirFS(filepath.Join(home, ".ashley")), Base: ashley.Assets}
	}
	catalog := skills.Catalog{Source: source}
	openUI := func(screen string) error {
		return tui.Run(tui.Options{Catalog: catalog, Root: *root, Screen: screen}, stdout)
	}
	if len(args) == 0 || *interactive {
		return openUI("hub")
	}
	if (args[0] == "sessions" && len(args) == 1) || args[0] == "vibe" || args[0] == "create" {
		if len(args) != 1 {
			return fmt.Errorf("%s does not accept arguments", args[0])
		}
		return openUI(args[0])
	}
	if args[0] == "history" && ((len(args) == 2 && args[1] == "browse") || (len(args) == 3 && args[1] == "show" && args[2] == "--tui")) {
		return openUI("history")
	}

	switch args[0] {
	case "update":
		return updateCommand(args[1:], *root, catalog, stdout, stderr)
	case "install", "uninstall", "upgrade":
		return installCommand(args[0], args[1:], catalog, stdout, stderr)
	case "run", "pipe":
		return runCommand(args[0], args[1:], catalog, stdout, stderr)
	case "sessions", "attach", "logs", "kill":
		return sessionCommand(args[0], args[1:], stdout, stderr)
	case "__execute":
		if len(args) != 2 {
			return fmt.Errorf("session supervisor requires a job file")
		}
		job, err := execution.LoadJob(args[1])
		if err != nil {
			return err
		}
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
		defer cancel()
		return execution.Run(ctx, job, os.Stdin, stdout, stderr)
	case "history":
		return historyCommand(args[1:], stdout, stderr)
	case "agent", "config":
		return preferences(args[0], args[1:], stdout, stderr)
	case "help":
		fmt.Fprint(stdout, help)
		return nil
	case "version":
		if len(args) != 1 {
			return fmt.Errorf("version does not accept arguments")
		}
		fmt.Fprintln(stdout, "ashley "+ashley.Version())
		return nil
	case "list":
		if len(args) != 1 {
			return fmt.Errorf("list does not accept arguments")
		}
		names, err := catalog.Names()
		if err != nil {
			return err
		}
		p := present(stdout)
		p.heading("Skills")
		var rows [][]string
		for _, name := range names {
			def, err := catalog.Load(name)
			if err != nil {
				return err
			}
			rows = append(rows, []string{name, def.Description})
		}
		p.table([]string{"Skill", "Description"}, rows)
		p.section("Next")
		p.line("ash prompt <skill> · Preview a prompt")
		p.line("ash run <skill>    · Start coding")
		return nil
	case "detect":
		if len(args) > 2 {
			return fmt.Errorf("detect accepts one directory")
		}
		dir, err := os.Getwd()
		if err != nil {
			return err
		}
		if len(args) == 2 {
			dir, err = filepath.Abs(args[1])
			if err != nil {
				return err
			}
		}
		info, err := os.Stat(dir)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("not a directory: %s", dir)
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(project.Detect(dir))
	case "generate", "prompt":
		command := flag.NewFlagSet(args[0], flag.ContinueOnError)
		command.SetOutput(stderr)
		projectDir := command.String("project", "", "Render templates for a project directory")
		var output *string
		if args[0] == "generate" {
			output = command.String("output", *root, "Output root (default: current directory or --root)")
		}
		if err := command.Parse(args[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil
			}
			return err
		}
		var context map[string]any
		if *projectDir != "" {
			dir, err := filepath.Abs(*projectDir)
			if err != nil {
				return err
			}
			info, err := os.Stat(dir)
			if err != nil {
				return err
			}
			if !info.IsDir() {
				return fmt.Errorf("not a directory: %s", dir)
			}
			context = project.Detect(dir)
		}
		if args[0] == "prompt" {
			if command.NArg() == 0 {
				return fmt.Errorf("prompt requires a skill name")
			}
			document, err := catalog.Document(command.Arg(0))
			if err != nil {
				return err
			}
			text, err := skills.Prompt(document, strings.Join(command.Args()[1:], " "), context)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(stdout, text)
			return err
		}
		if command.NArg() != 0 {
			return fmt.Errorf("generate does not accept positional arguments")
		}
		names, err := catalog.Names()
		if err != nil {
			return err
		}
		// Render everything before writing, so a broken definition cannot leave
		// only the first half of the catalog regenerated.
		results := make([]skills.Result, 0, len(names))
		for _, name := range names {
			result, err := catalog.Assemble(name, context)
			if err != nil {
				return err
			}
			if !filepath.IsLocal(result.Output) {
				return fmt.Errorf("output must stay within the output root: %s", result.Output)
			}
			results = append(results, result)
		}
		destination := *output
		if destination == "" {
			destination = "."
		}
		// Root prevents custom output paths or existing symlinks from escaping
		// the explicitly selected output directory.
		if err := os.MkdirAll(destination, 0755); err != nil {
			return err
		}
		outputRoot, err := os.OpenRoot(destination)
		if err != nil {
			return err
		}
		defer outputRoot.Close()
		p := present(stdout)
		p.heading("Generate")
		for _, result := range results {
			if err := outputRoot.MkdirAll(filepath.Dir(result.Output), 0755); err != nil {
				return err
			}
			if err := outputRoot.WriteFile(result.Output, []byte(result.Content), 0644); err != nil {
				return err
			}
			p.line(result.Output)
		}
		p.success(fmt.Sprintf("Generated: %d skills", len(results)))
		return nil
	default:
		return fmt.Errorf("unknown command %q (see ash --help)", args[0])
	}
}
