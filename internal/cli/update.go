package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	ashley "github.com/LBYPatrick/ashley"
	"github.com/LBYPatrick/ashley/internal/install"
	"github.com/LBYPatrick/ashley/internal/selfupdate"
	"github.com/LBYPatrick/ashley/internal/skills"
)

func updateCommand(args []string, sourceRoot string, catalog skills.Catalog, stdout, stderr io.Writer) error {
	updater := selfupdate.Updater{Repo: os.Getenv("ASHLEY_REPO"), Platform: runtime.GOOS, Arch: runtime.GOARCH}
	return updateWithUpdater(args, sourceRoot, catalog, updater, stdout, stderr)
}

func updateWithUpdater(args []string, sourceRoot string, catalog skills.Catalog, updater selfupdate.Updater, stdout, stderr io.Writer) error {
	p := present(stdout)
	p.heading("Update")
	flags := flag.NewFlagSet("update", flag.ContinueOnError)
	flags.SetOutput(stderr)
	version := flags.String("version", "", "Install a specific release version")
	check := flags.Bool("check", false, "Check latest release without changing files")
	destinationDir := flags.String("install-dir", os.Getenv("ASHLEY_INSTALL_DIR"), "Install into a directory instead of replacing this executable")
	branch := flags.String("branch", "main", "Release channel (main)")
	skip := flags.Bool("skip-tools", false, "Do not upgrade agent CLIs")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("update does not accept positional arguments")
	}
	if *branch != "main" && sourceRoot == "" {
		return fmt.Errorf("binary releases track main; select a published prerelease with --version (branch builds require a developer checkout)")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	var err error
	if sourceRoot != "" {
		if *version != "" {
			return fmt.Errorf("choose either a development checkout or a release --version")
		}
		*version = ashley.Version()
	} else if *version == "" {
		*version, err = updater.Latest(ctx)
	} else {
		*version, err = selfupdate.Version(*version)
	}
	if err != nil {
		return err
	}
	if sourceRoot != "" {
		p.field("Checkout", sourceRoot)
		p.field("Branch", *branch)
	} else {
		p.field("Installed", ashley.Version())
		p.field("Release", *version)
	}
	if *check {
		return nil
	}
	destination, err := os.Executable()
	if err != nil {
		return err
	}
	if *destinationDir != "" {
		destination, err = filepath.Abs(filepath.Join(*destinationDir, "ash"))
		if err != nil {
			return err
		}
	}
	if sourceRoot != "" {
		if err := selfupdate.SourceUpdate(ctx, sourceRoot, destination, *branch, stdout, stderr); err != nil {
			return err
		}
		p.success("Installed development build: " + destination)
	} else if *version != ashley.Version() || *destinationDir != "" {
		data, err := updater.Fetch(ctx, *version)
		if err != nil {
			return err
		}
		if err := selfupdate.Install(ctx, destination, *version, data); err != nil {
			return err
		}
		p.success("Installed binary: " + destination)
	} else {
		p.success("Ashley is already up to date.")
	}
	// Run the new executable so refreshed skills come from the new embedded catalog.
	refreshArgs := []string{"install", "--skills-only"}
	if sourceRoot != "" {
		generate := exec.CommandContext(ctx, destination, "--root", sourceRoot, "generate", "--output", sourceRoot)
		generate.Stdout = stdout
		generate.Stderr = stderr
		if err := generate.Run(); err != nil {
			return fmt.Errorf("development build installed, but generation failed: %w", err)
		}
		refreshArgs = append([]string{"--root", sourceRoot}, refreshArgs...)
	}
	refresh := exec.CommandContext(ctx, destination, refreshArgs...)
	refresh.Stdin = os.Stdin
	refresh.Stdout = stdout
	refresh.Stderr = stderr
	if err := refresh.Run(); err != nil {
		return fmt.Errorf("binary installed, but skill refresh failed: %w", err)
	}
	skipEnv := strings.ToLower(strings.TrimSpace(os.Getenv("SKIP_TOOL")))
	if *skip || skipEnv == "true" || skipEnv == "1" || skipEnv == "yes" {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	keys, err := (install.Installer{Home: home, Catalog: catalog}).Resolve(nil)
	if err != nil {
		return err
	}
	upgrade := exec.CommandContext(ctx, destination, append([]string{"upgrade"}, keys...)...)
	upgrade.Stdin = os.Stdin
	upgrade.Stdout = stdout
	upgrade.Stderr = stderr
	return upgrade.Run()
}
