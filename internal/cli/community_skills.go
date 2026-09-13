package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

func communitySkillCommands(keys []string) [][]string {
	targets := make([]string, len(keys))
	for i, key := range keys {
		switch key {
		case "claude":
			targets[i] = "claude-code"
		default:
			targets[i] = key
		}
	}
	var commands [][]string
	for _, bundle := range []struct{ repo, skill string }{{"emilkowalski/skills", "*"}, {"vercel-labs/skills", "find-skills"}} {
		args := []string{"add", bundle.repo, "--global", "--yes", "--skill", bundle.skill, "--agent"}
		commands = append(commands, append(args, targets...))
	}
	return commands
}

func installCommunitySkills(keys []string, cfg *automatedInstall, stdout, stderr io.Writer) error {
	installCLI := envEnabled("ASHLEY_INSTALL_SKILLS")
	selected := false
	if cfg != nil {
		installCLI = cfg.InstallSkills
		selected = cfg.CommunitySkills
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
	if cfg == nil {
		// A read-only probe includes package-manager global bins and managed paths.
		if !installCLI && runSkills(nil, strings.NewReader(""), io.Discard, io.Discard, []string{"ASHLEY_SKILLS_CHECK=1"}) != nil {
			return nil
		}
		input := os.Stdin
		if !isatty.IsTerminal(input.Fd()) {
			tty, err := os.Open("/dev/tty")
			if err != nil {
				return nil
			}
			defer tty.Close()
			input = tty
		}
		fmt.Fprintf(stdout, "Install all emilkowalski/skills plus find-skills for %s? [y/N] ", strings.Join(keys, ", "))
		answer, _ := bufio.NewReader(input).ReadString('\n')
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "y", "yes":
			selected = true
		}
	}
	if selected && cfg == nil {
		env = append(env, "ASHLEY_INSTALL_SKILLS=1")
	}
	if !selected {
		return nil
	}
	for _, args := range communitySkillCommands(keys) {
		if err := runSkills(args, strings.NewReader(""), stdout, stderr, env); err != nil {
			return fmt.Errorf("install community skills: %w", err)
		}
	}
	return nil
}
