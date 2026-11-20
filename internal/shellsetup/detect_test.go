package shellsetup

import (
	"context"
	"errors"
	"testing"
)

func TestParseShellProcess(t *testing.T) {
	for _, test := range []struct {
		command     string
		shell       string
		login       bool
		interactive bool
	}{
		{command: "bash", shell: "bash", interactive: true},
		{command: "/bin/zsh", shell: "zsh", interactive: true},
		{command: "-bash", shell: "bash", login: true, interactive: true},
		{command: "-zsh", shell: "zsh", login: true, interactive: true},
		{command: "/bin/bash -l", shell: "bash", login: true, interactive: true},
		{command: "bash -il", shell: "bash", login: true, interactive: true},
		{command: "bash --interactive --login", shell: "bash", login: true, interactive: true},
		{command: "zsh -f -i", shell: "zsh", interactive: true},
		{command: "fish --login --interactive", shell: "fish", login: true, interactive: true},
		{command: "sh -i", shell: "sh", interactive: true},
		{command: "bash script.sh", shell: "bash"},
		{command: "bash ./install.sh -i --login", shell: "bash"},
		{command: "bash -c echo hello", shell: "bash"},
		{command: "bash -lc echo hello", shell: "bash", login: true},
		{command: "bash -c echo -i --login", shell: "bash"},
		{command: "zsh --login -c install", shell: "zsh", login: true},
		{command: "fish --command=install", shell: "fish"},
		{command: "fish -c install", shell: "fish"},
		{command: "bash -ic echo hello", shell: "bash", interactive: true},
		{command: "bash -lic echo hello", shell: "bash", login: true, interactive: true},
		{command: "bash --interactive --command echo hello", shell: "bash", interactive: true},
		{command: "bash -s", shell: "bash"},
		{command: "bash -is", shell: "bash", interactive: true},
		{command: "bash -- script.sh", shell: "bash"},
		{command: "bash --", shell: "bash", interactive: true},
		{command: "bash +i", shell: "bash"},
		{command: "bash --noprofile --norc", shell: "bash", interactive: true},
		{command: "bash --rcfile /tmp/rcfile -i", shell: "bash", interactive: true},
		{command: "bash -o posix script.sh", shell: "bash"},
		{command: "bash -Oextglob", shell: "bash", interactive: true},
		{command: "bash -oignoreeof script.sh", shell: "bash"},
		{command: "bash -Ci", shell: "bash", interactive: true},
		{command: "fish -C source-init.fish", shell: "fish", interactive: true},
		{command: "fish --init-command source-init.fish", shell: "fish", interactive: true},
		{command: "bash --help", shell: "bash"},
		{command: "fish --version", shell: "fish"},
		{command: "env bash -i"},
		{command: "tmux"},
		{command: ""},
	} {
		t.Run(test.command, func(t *testing.T) {
			shell, login, interactive := parseShellProcess(test.command)
			if shell != test.shell || login != test.login || interactive != test.interactive {
				t.Fatalf("got (%q, %t, %t), want (%q, %t, %t)", shell, login, interactive, test.shell, test.login, test.interactive)
			}
		})
	}
}

func TestDetectFindsNearestInteractiveShell(t *testing.T) {
	for _, test := range []struct {
		name      string
		processes map[int]parentProcess
		fallback  string
		shell     string
		login     bool
	}{
		{
			name: "installer wrapper below login shell",
			processes: map[int]parentProcess{
				10: {parent: 20, command: "bash /tmp/install.sh"},
				20: {parent: 30, command: "-zsh"},
			},
			fallback: "/bin/bash", shell: "zsh", login: true,
		},
		{
			name: "command wrappers below interactive shell",
			processes: map[int]parentProcess{
				10: {parent: 20, command: "bash -c install"},
				20: {parent: 30, command: "bash -lc install"},
				30: {parent: 40, command: "fish -i"},
			},
			fallback: "/bin/zsh", shell: "fish",
		},
		{
			name: "nested interactive shell takes priority",
			processes: map[int]parentProcess{
				10: {parent: 20, command: "bash /tmp/install.sh"},
				20: {parent: 30, command: "bash -i"},
				30: {parent: 1, command: "-zsh"},
			},
			fallback: "/bin/zsh", shell: "bash",
		},
		{
			name: "login option controls bash startup",
			processes: map[int]parentProcess{
				10: {parent: 1, command: "/opt/homebrew/bin/bash --login --interactive"},
			},
			fallback: "/bin/zsh", shell: "bash", login: true,
		},
		{
			name:     "unavailable process information",
			fallback: "/usr/local/bin/fish", shell: "fish",
		},
		{
			name:     "unsupported environment shell remains identifiable",
			fallback: "/usr/bin/tcsh", shell: "tcsh",
		},
		{
			name: "missing environment shell",
		},
		{
			name: "only installer wrappers",
			processes: map[int]parentProcess{
				10: {parent: 20, command: "bash /tmp/install.sh"},
				20: {parent: 1, command: "sh -c install"},
			},
			fallback: "/bin/bash", shell: "bash",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			lookup := func(_ context.Context, pid int) (parentProcess, error) {
				if process, ok := test.processes[pid]; ok {
					return process, nil
				}
				return parentProcess{}, errors.New("ps unavailable")
			}
			shell, login := detectFromProcesses(10, test.fallback, lookup)
			if shell != test.shell || login != test.login {
				t.Fatalf("got (%q, %t), want (%q, %t)", shell, login, test.shell, test.login)
			}
		})
	}
}

func TestDetectBoundsProcessLookup(t *testing.T) {
	for _, cycle := range []bool{false, true} {
		calls := 0
		lookup := func(ctx context.Context, pid int) (parentProcess, error) {
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("parent lookup has no time limit")
			}
			calls++
			if cycle {
				return parentProcess{parent: pid, command: "bash install.sh"}, nil
			}
			return parentProcess{parent: pid + 1, command: "bash install.sh"}, nil
		}
		shell, login := detectFromProcesses(10, "/bin/zsh", lookup)
		if calls > 32 || calls < 1 || shell != "zsh" || login {
			t.Fatal("unbounded process walk or incorrect fallback", calls, shell, login)
		}
		if cycle && calls != 1 {
			t.Fatal("self-referencing parent did not stop the process walk", calls)
		}
	}
}
