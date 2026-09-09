package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/LBYPatrick/ashley/internal/agents"
	"github.com/LBYPatrick/ashley/internal/config"
	"github.com/LBYPatrick/ashley/internal/install"
	"github.com/mattn/go-isatty"
)

func chooseInstallAgents(installer install.Installer, stdout io.Writer) []string {
	for _, key := range agents.Keys() {
		if installer.HasSkills(key) {
			return nil
		}
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
	prefs, err := config.User()
	if err != nil {
		return nil
	}
	saved := prefs.LoadAgent()
	p := present(stdout)
	p.section("Choose your coding agent")
	p.line("Enter a number, or press Enter to keep the default.")
	fmt.Fprintln(stdout)
	for i, key := range agents.Keys() {
		suffix := ""
		if key == saved {
			suffix = " (default)"
		}
		fmt.Fprintf(stdout, "    %d  %s%s\n", i+1, agents.Get(key).Label, suffix)
	}
	fmt.Fprintln(stdout, "    6  All agents")
	fmt.Fprintln(stdout)
	fmt.Fprint(stdout, "  Choice [Enter keeps the default]: ")
	answer, err := bufio.NewReader(input).ReadString('\n')
	if err != nil {
		return nil
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer == "all" || answer == "6" {
		return agents.Keys()
	}
	if answer == "both" {
		return []string{"claude", "codex"}
	}
	if index, err := strconv.Atoi(answer); err == nil && index >= 1 && index <= len(agents.Keys()) {
		return []string{agents.Keys()[index-1]}
	}
	if agents.Valid(answer) {
		return []string{answer}
	}
	return nil
}
