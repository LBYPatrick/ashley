package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/LBYPatrick/ashley/internal/agents"
	"github.com/LBYPatrick/ashley/internal/config"
	"github.com/LBYPatrick/ashley/internal/install"
	"github.com/LBYPatrick/ashley/internal/skills"
	"github.com/LBYPatrick/ashley/internal/upgrade"
)

func installOptions(args []string) (keys []string, skillsOnly, check bool, err error) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--skills-only":
			skillsOnly = true
			continue
		case "--check":
			check = true
			continue
		case "--agent":
			i++
			if i >= len(args) {
				return nil, false, false, fmt.Errorf("--agent requires a name")
			}
			arg = args[i]
		case "-c":
			arg = "claude"
		case "-o":
			arg = "codex"
		}
		arg = strings.TrimPrefix(strings.TrimPrefix(arg, "--agent="), "--")
		switch arg {
		case "all":
			keys = append(keys, agents.Keys()...)
		case "both":
			keys = append(keys, "claude", "codex")
		default:
			if !agents.Valid(arg) {
				return nil, false, false, fmt.Errorf("unknown agent or option: %s", arg)
			}
			keys = append(keys, agents.Get(arg).Key)
		}
	}
	return keys, skillsOnly, check, nil
}
func installCommand(command string, args []string, catalog skills.Catalog, stdout, stderr io.Writer) (err error) {
	legacyRoot := ""
	verbose := false
	var options []string
	for index := 0; index < len(args); index++ {
		if args[index] == "--verbose" {
			verbose = true
			continue
		}
		if args[index] != "--legacy-root" {
			options = append(options, args[index])
			continue
		}
		index++
		if command != "install" || index >= len(args) || args[index] == "" {
			return fmt.Errorf("--legacy-root requires a checkout path and is only supported by install")
		}
		var err error
		legacyRoot, err = filepath.Abs(args[index])
		if err != nil {
			return err
		}
		if info, err := os.Stat(filepath.Join(legacyRoot, "skills")); err != nil || !info.IsDir() {
			return fmt.Errorf("invalid legacy Ashley checkout: %s", legacyRoot)
		}
	}
	keys, skillsOnly, check, err := installOptions(options)
	if err != nil {
		return err
	}
	if check && command != "upgrade" {
		return fmt.Errorf("--check is only supported by upgrade")
	}
	if skillsOnly && command != "install" {
		return fmt.Errorf("--skills-only is only supported by install")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	p := present(stdout)
	p.heading(strings.ToUpper(command[:1]) + command[1:])
	installer := install.Installer{Home: home, Catalog: catalog, LegacyRoot: legacyRoot}
	if command == "uninstall" {
		result, err := installer.Uninstall(keys)
		if err != nil {
			return err
		}
		p.success(fmt.Sprintf("Removed %d Ashley skill links.", result.Removed))
		return nil
	}
	if command == "upgrade" && len(keys) == 0 {
		prefs, e := config.User()
		if e != nil {
			return e
		}
		keys = []string{prefs.LoadAgent()}
	}
	if command == "install" && len(keys) == 0 {
		keys = chooseInstallAgents(installer, stdout)
	}
	keys, err = installer.Resolve(keys)
	if err != nil {
		return err
	}
	vendorOut := &indentedWriter{out: stdout, start: true}
	vendorErr := &indentedWriter{out: stderr, start: true}
	manager := upgrade.Manager{Run: upgrade.CommandRunner(vendorOut, vendorErr)}
	ctx := context.Background()
	if command == "upgrade" {
		failed := []string{}
		for _, key := range keys {
			status := (upgrade.Manager{Run: upgrade.CommandRunner(io.Discard, io.Discard)}).Detect(ctx, key)
			p.section(status.Agent.Label)
			p.field("Version", status.Version)
			p.field("Source", status.Source)
			p.field("Executable", status.Path)
			if check {
				continue
			}
			if err := manager.Upgrade(ctx, key); err != nil {
				fmt.Fprintln(stderr, err)
				failed = append(failed, key)
			}
		}
		if len(failed) > 0 {
			return fmt.Errorf("upgrade failed for: %s", strings.Join(failed, ", "))
		}
		return nil
	}
	logDir := filepath.Join(home, ".ashley", "logs")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return err
	}
	log, err := os.CreateTemp(logDir, "install-*.log")
	if err != nil {
		return err
	}
	defer log.Close()
	// Display the recovery log even when a vendor installer or generation fails.
	defer func() {
		if err != nil {
			fmt.Fprintln(log, "Error:", err)
		}
		p.field("Full log", log.Name())
		fmt.Fprintln(stdout)
	}()
	installer.Log = log
	if verbose {
		installer.Log = io.MultiWriter(log, &indentedWriter{out: stdout, start: true})
	}
	manager.Run = upgrade.CommandRunner(io.MultiWriter(vendorOut, log), io.MultiWriter(vendorErr, log))
	if !skillsOnly {
		p.section("Agent setup")
		for _, key := range keys {
			p.line(agents.Get(key).Label)
			if err := manager.Ensure(ctx, key); err != nil {
				return fmt.Errorf("install %s: %w", key, err)
			}
			p.field("Ready", agents.Get(key).Label)
		}
	}
	p.section("Sync skills")
	if skillsOnly {
		p.line("Regenerate and install for selected agents")
	}
	result, err := installer.Install(keys)
	if err != nil {
		return err
	}
	perAgent := (result.Installed + result.Skipped) / len(keys)
	for _, key := range keys {
		p.field(agents.Get(key).Label, fmt.Sprintf("%d skills · %s", perAgent, agents.SkillsDir(key, home, os.Getenv)))
	}
	agentNoun := "agents"
	if len(keys) == 1 {
		agentNoun = "agent"
	}
	p.success(fmt.Sprintf("Ready · %d skills for %d %s", perAgent, len(keys), agentNoun))
	p.field("Next", "ash")
	return nil
}
