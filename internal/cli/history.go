package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/LBYPatrick/ashley/internal/agents"
	"github.com/LBYPatrick/ashley/internal/history"
)

func historyCommand(args []string, stdout, stderr io.Writer) error {
	command := "show"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command, args = args[0], args[1:]
	}
	flags := flag.NewFlagSet("history "+command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	var filter history.Filter
	limit, offset := 20, 0
	jsonOutput, yes := false, false
	switch command {
	case "show", "stats":
		flags.StringVar(&filter.Skill, "skill", "", "Filter by skill")
		flags.StringVar(&filter.Agent, "agent", "", "Filter by coding agent")
		flags.BoolVar(&jsonOutput, "json", false, "Print JSON")
		if command == "show" {
			flags.StringVar(&filter.Search, "search", "", "Search skill, question, or directory")
			flags.StringVar(&filter.Search, "s", "", "Search")
			flags.IntVar(&limit, "limit", 20, "Maximum entries")
			flags.IntVar(&limit, "n", 20, "Maximum entries")
			flags.IntVar(&offset, "offset", 0, "Entries to skip")
		}
	case "clear":
		flags.BoolVar(&yes, "yes", false, "Delete all entries without prompting")
	case "prune", "info":
	default:
		return fmt.Errorf("unknown history command: %s", command)
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if filter.Agent != "" && !agents.Valid(filter.Agent) {
		return fmt.Errorf("unknown coding agent: %s", filter.Agent)
	}
	if filter.Agent != "" {
		filter.Agent = agents.Get(filter.Agent).Key
	}
	if offset < 0 || limit < 0 {
		return fmt.Errorf("limit and offset must be nonnegative")
	}
	days := 0
	if command == "prune" {
		if flags.NArg() != 1 {
			return fmt.Errorf("prune requires a number of days")
		}
		var err error
		days, err = strconv.Atoi(flags.Arg(0))
		if err != nil || days < 0 {
			return fmt.Errorf("days must be a nonnegative integer")
		}
	} else if flags.NArg() != 0 {
		return fmt.Errorf("history %s does not accept positional arguments", command)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	path := history.Path(home, runtime.GOOS, os.Getenv("XDG_DATA_HOME"))
	store, err := history.Open(path)
	if err != nil {
		return err
	}
	defer store.Close()
	switch command {
	case "show":
		entries, err := store.Query(filter, limit, offset)
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(stdout).Encode(entries)
		}
		if len(entries) == 0 {
			fmt.Fprintln(stdout, "No history entries found.")
			return nil
		}
		fmt.Fprintf(stdout, "%5s  %-19s  %-12s  %-12s  %-30s  %s\n", "ID", "Time", "Skill", "Agent", "Dir", "Question")
		for _, v := range entries {
			dir := []rune(v.CWD)
			if len(dir) > 30 {
				dir = append([]rune("…"), dir[len(dir)-29:]...)
			}
			skill := v.Skill
			if v.Detached {
				skill += " ⇢"
			}
			fmt.Fprintf(stdout, "%5d  %-19s  %-12s  %-12s  %-30s  %s\n", v.ID, v.TimeDisplay(), skill, v.AgentType, string(dir), v.QuestionShort())
		}
	case "stats":
		stats, err := store.Stats(filter)
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(stdout).Encode(stats)
		}
		fmt.Fprintf(stdout, "Ashley Analytics\nTotal invocations: %d\n", stats.Total)
		if len(stats.TopSkills) > 0 {
			fmt.Fprintln(stdout, "Top Skills")
			for _, v := range stats.TopSkills {
				fmt.Fprintf(stdout, "  %-14s %4d  %s\n", v.Name, v.Count, strings.Repeat("█", min(v.Count, 30)))
			}
		}
		if len(stats.ByAgent) > 0 {
			fmt.Fprintln(stdout, "By Agent")
			for _, v := range stats.ByAgent {
				fmt.Fprintf(stdout, "  %-14s %4d  %s\n", agents.Get(v.Name).Label, v.Count, strings.Repeat("█", min(v.Count, 30)))
			}
		}
	case "prune":
		n, err := store.Prune(days)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Pruned %d entries older than %d days.\n", n, days)
	case "clear":
		if !yes {
			fmt.Fprint(stdout, "Delete ALL history entries? [y/N]: ")
			answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
			if err != nil {
				return fmt.Errorf("confirmation required; use --yes: %w", err)
			}
			if strings.ToLower(strings.TrimSpace(answer)) != "y" && strings.ToLower(strings.TrimSpace(answer)) != "yes" {
				fmt.Fprintln(stdout, "Cancelled.")
				return nil
			}
		}
		n, err := store.Clear()
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Cleared %d history entries.\n", n)
	case "info":
		count, err := store.Count(history.Filter{})
		if err != nil {
			return err
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Database: %s\nSize: %d bytes\nEntries: %d\n", path, info.Size(), count)
	}
	return nil
}
