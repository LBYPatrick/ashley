package shellsetup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type parentProcess struct {
	parent  int
	command string
}

type processLookup func(context.Context, int) (parentProcess, error)

// Detect finds the nearest interactive parent shell and its login mode.
// If process information is unavailable, it uses SHELL without login mode.
func Detect() (shell string, login bool) {
	return detectFromProcesses(os.Getppid(), os.Getenv("SHELL"), readParentProcess)
}

func detectFromProcesses(pid int, fallback string, lookup processLookup) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for attempts := 0; pid > 1 && attempts < 32 && ctx.Err() == nil; attempts++ {
		process, err := lookup(ctx, pid)
		if err != nil {
			break
		}
		if shell, login, interactive := parseShellProcess(process.command); interactive {
			return shell, login
		}
		if process.parent == pid {
			break
		}
		pid = process.parent
	}
	if fallback == "" {
		return "", false
	}
	return strings.TrimPrefix(filepath.Base(fallback), "-"), false
}

func readParentProcess(ctx context.Context, pid int) (parentProcess, error) {
	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	output, err := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "ppid=", "-o", "args=").Output()
	if err != nil {
		return parentProcess{}, err
	}
	fields := strings.Fields(string(output))
	if len(fields) < 2 {
		return parentProcess{}, fmt.Errorf("ps returned no parent process information")
	}
	parent, err := strconv.Atoi(fields[0])
	if err != nil {
		return parentProcess{}, fmt.Errorf("parse parent PID: %w", err)
	}
	return parentProcess{parent: parent, command: strings.Join(fields[1:], " ")}, nil
}

// Parse only startup options. Arguments after a command or script belong to it.
func parseShellProcess(command string) (shell string, login, interactive bool) {
	args := strings.Fields(command)
	if len(args) == 0 {
		return "", false, false
	}
	name := filepath.Base(args[0])
	login = strings.HasPrefix(name, "-")
	shell = strings.TrimPrefix(name, "-")
	switch shell {
	case "bash", "zsh", "fish", "sh":
	default:
		return "", false, false
	}
	var explicit, noninteractive bool
	for index := 1; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			noninteractive = index+1 < len(args)
			break
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "+") {
			noninteractive = true
			break
		}
		if strings.HasPrefix(arg, "--") {
			switch {
			case arg == "--interactive":
				explicit, interactive = true, true
			case arg == "--login":
				login = true
			case arg == "--command" || strings.HasPrefix(arg, "--command="):
				noninteractive = true
			case arg == "--help" || arg == "--version":
				return shell, login, false
			case arg == "--rcfile" || arg == "--init-file" || arg == "--init-command" || arg == "--features":
				index++
			}
		} else {
			enabled := arg[0] == '-'
		options:
			for offset, option := range arg[1:] {
				switch option {
				case 'i':
					explicit, interactive = true, enabled
				case 'l':
					login = enabled
				case 'c', 's':
					noninteractive = enabled
				case 'o', 'O':
					if offset == len(arg)-2 {
						index++
					}
					break options
				case 'C':
					if shell == "fish" {
						if offset == len(arg)-2 {
							index++
						}
						break options
					}
				}
			}
		}
		if noninteractive {
			break
		}
	}
	if explicit {
		return shell, login, interactive
	}
	return shell, login, !noninteractive
}
