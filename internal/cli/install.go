package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
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
func installCommand(command string, args []string, catalog skills.Catalog, stdout, stderr io.Writer) error {
	legacyRoot := ""
	var options []string
	for index := 0; index < len(args); index++ {
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
	installer := install.Installer{Home: home, Catalog: catalog, Log: stdout, LegacyRoot: legacyRoot}
	if command == "uninstall" {
		result, err := installer.Uninstall(keys)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Removed %d Ashley skill links.\n", result.Removed)
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
	manager := upgrade.Manager{Run: upgrade.CommandRunner(stdout, stderr)}
	ctx := context.Background()
	if command == "upgrade" {
		failed := []string{}
		for _, key := range keys {
			status := manager.Detect(ctx, key)
			fmt.Fprintf(stdout, "%s: %s (%s) %s\n", status.Agent.Label, status.Version, status.Source, status.Path)
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
	if !skillsOnly {
		for _, key := range keys {
			if err := manager.Ensure(ctx, key); err != nil {
				return fmt.Errorf("install %s: %w", key, err)
			}
		}
	}
	result, err := installer.Install(slices.Clone(keys))
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Installed %d skill links; preserved %d existing paths.\n", result.Installed, result.Skipped)
	return nil
}
