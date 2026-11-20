package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/LBYPatrick/ashley/internal/community"
	"github.com/LBYPatrick/ashley/internal/install"
	"github.com/mattn/go-isatty"
)

func communitySkillCommands(keys []string) [][]string {
	var commands [][]string
	for _, bundle := range community.Bundles() {
		commands = append(commands, communitySkillCommand(bundle, keys))
	}
	return commands
}

func communitySkillCommand(bundle community.Bundle, keys []string) []string {
	targets := make([]string, len(keys))
	for i, key := range keys {
		switch key {
		case "claude":
			targets[i] = "claude-code"
		default:
			targets[i] = key
		}
	}
	return append([]string{"add", bundle.Source, "--global", "--yes", "--skill", bundle.Skill, "--agent"}, targets...)
}

func applyCommunityBundles(desired []string, stdout, stderr io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	installer := install.Installer{Home: home}
	keys, err := installer.Resolve(nil)
	if err != nil {
		return err
	}
	env := []string{"ASHLEY_INSTALL_SKILLS=1", "ASHLEY_AUTOMATED=1", "CI=1", "GIT_TERMINAL_PROMPT=0"}
	return manageCommunityBundles(home, keys, desired, stdout, stderr, env)
}

func manageCommunityBundles(home string, keys, desired []string, stdout, stderr io.Writer, env []string) error {
	return community.Apply(home, keys, desired, func(bundle community.Bundle, stage string) error {
		return runSkills(communitySkillCommand(bundle, keys), strings.NewReader(""), stdout, stderr,
			append(slices.Clone(env), "ASHLEY_SKILLS_STAGE="+stage))
	}, stdout)
}

func installCommunitySkills(keys []string, cfg *automatedInstall, stdout, stderr io.Writer) error {
	var input io.Reader
	if cfg == nil {
		if isatty.IsTerminal(os.Stdin.Fd()) {
			input = os.Stdin
		} else if tty, err := os.Open("/dev/tty"); err == nil {
			defer tty.Close()
			input = tty
		}
	}
	return installCommunitySkillsWithInput(keys, cfg, input, stdout, stderr)
}

func installCommunitySkillsWithInput(keys []string, cfg *automatedInstall, input io.Reader, stdout, stderr io.Writer) error {
	installCLI := envEnabled("ASHLEY_INSTALL_SKILLS")
	selected := false
	if cfg != nil {
		installCLI = cfg.InstallSkills
		selected = cfg.CommunitySkills
	} else if input != nil {
		fmt.Fprintf(stdout, "Install all emilkowalski/skills, GoogleChrome/modern-web-guidance, and find-skills for %s? (recommended) [Y/n] ", strings.Join(keys, ", "))
		answer, err := bufio.NewReader(input).ReadString('\n')
		if err == nil {
			switch strings.ToLower(strings.TrimSpace(answer)) {
			case "", "y", "yes":
				selected = true
				installCLI = true
			}
		}
	}
	env := []string{}
	if installCLI {
		env = append(env, "ASHLEY_INSTALL_SKILLS=1")
	} else if cfg != nil {
		env = append(env, "ASHLEY_INSTALL_SKILLS=0")
	}
	if cfg != nil {
		env = append(env, "ASHLEY_AUTOMATED=1", "CI=1", "GIT_TERMINAL_PROMPT=0", "GIT_SSH_COMMAND=ssh -oBatchMode=yes")
	}
	if installCLI {
		if err := runSkills(nil, strings.NewReader(""), stdout, stderr, append(env, "ASHLEY_SKILLS_ENSURE=1")); err != nil {
			return err
		}
	}
	if !selected {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	bundles := community.Bundles()
	desired := make([]string, 0, len(bundles))
	for _, bundle := range bundles {
		desired = append(desired, bundle.ID)
	}
	return manageCommunityBundles(home, keys, desired, stdout, stderr, env)
}
