package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/LBYPatrick/ashley/internal/agents"
	"github.com/LBYPatrick/ashley/internal/sessions"
	tea "github.com/charmbracelet/bubbletea"
)

type operation struct {
	kind, output, destination string
	target                    string
	busy                      bool
	err                       error
	elapsed                   time.Duration
	cancel                    context.CancelFunc
}
type operationFinished struct {
	job     *operation
	output  string
	err     error
	elapsed time.Duration
}

// Retain bounded command output without handing the terminal away for quick work.
type outputTail struct{ data []byte }

func (b *outputTail) Write(p []byte) (int, error) {
	n := len(p)
	b.data = append(b.data, p...)
	if len(b.data) > 128*1024 {
		b.data = append([]byte(nil), b.data[len(b.data)-128*1024:]...)
	}
	return n, nil
}
func (m *Model) startOperation(kind string) tea.Cmd {
	if m.job != nil && m.job.busy {
		m.status = "A task is already running. Its result will stay available."
		return nil
	}
	destination := m.options.Root
	if destination == "" {
		destination, _ = os.Getwd()
	}
	args := []string{"generate", "--output", destination}
	if kind == "install" {
		destination = filepath.Join(m.options.Home, ".ashley", "generated")
		m.detectInstallAgents()
		if len(m.installAgents) == 0 {
			m.status = "No supported agents detected. Set up an agent CLI, then try again."
			return nil
		}
		args = []string{"install", "--skills-only"}
		for _, key := range m.installAgents {
			args = append(args, "--agent", key)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	job := &operation{kind: kind, busy: true, destination: destination, cancel: cancel}
	if kind == "install" {
		job.target = m.installLabels()
	}
	m.job = job
	m.screenScroll = 0
	m.status = ""
	root, home, capture := m.options.Root, m.options.Home, m.options.Background
	return func() tea.Msg {
		defer cancel()
		start := time.Now()
		var output string
		var err error
		if root != "" {
			args = append([]string{"--root", root}, args...)
		}
		if capture != nil {
			output, err = capture(ctx, args)
		} else {
			var executable string
			executable, err = os.Executable()
			if err == nil {
				cmd := exec.CommandContext(ctx, executable, args...)
				cmd.Env = append(os.Environ(), "HOME="+home)
				var tail outputTail
				cmd.Stdout = &tail
				cmd.Stderr = &tail
				err = cmd.Run()
				output = string(tail.data)
			}
		}
		return operationFinished{job, sessions.DisplayLog(output), err, time.Since(start)}
	}
}
func (m *Model) detectInstallAgents() {
	m.installAgents = nil
	for _, key := range agents.Keys() {
		if _, err := agents.FindBinary(agents.Get(key).Binary); err == nil {
			m.installAgents = append(m.installAgents, key)
		}
	}
}
func (m *Model) installLabels() string {
	labels := make([]string, 0, len(m.installAgents))
	for _, key := range m.installAgents {
		labels = append(labels, agents.Get(key).Label)
	}
	return strings.Join(labels, ", ")
}
func (m *Model) operationKey(key string) tea.Cmd {
	switch key {
	case "enter", "r":
		return m.startOperation(m.screen)
	case "i":
		if m.screen == "install" && (m.job == nil || !m.job.busy) {
			return m.execute("install", "--agent", m.agent)
		}
	case "up", "pgup":
		m.screenScroll = max(0, m.screenScroll-3)
	case "down", "pgdown":
		m.screenScroll = min(m.operationMaxScroll(), m.screenScroll+3)
	}
	return nil
}
func (m *Model) operationText() string {
	title := "Generate skills"
	description := "Assemble your skill definitions into ready-to-use prompts."
	action := "Enter Generate   ·   Esc Back"
	if m.screen == "install" {
		title = "Install skills"
		label := m.installLabels()
		if label == "" {
			label = "None — set up an agent CLI to get started."
		}
		description = "Install skills for every detected coding agent. Existing custom skills are preserved.\n\nDetected:  " + label + "\n\nI sets up " + agents.Get(m.agent).Label + " (chosen in Settings)."
		action = "Enter Install skills   ·   I Set up agent CLI   ·   Esc Back"
	}
	text := title + "\n\n" + description
	job := m.job
	if job == nil || job.kind != m.screen {
		return text + "\n\n" + action
	}
	if job.busy {
		return text + "\n\nWorking… You can return to the hub while this finishes.\n\nDestination\n" + job.destination
	}
	result := "✓ Skills generated"
	if m.screen == "install" {
		result = "✓ Skills installed for " + job.target
	}
	if job.err != nil {
		result = "Could not finish: " + job.err.Error()
	}
	text += "\n\n" + result + fmt.Sprintf(" · %s", job.elapsed.Round(time.Millisecond)) + "\n\nDestination\n" + job.destination
	if strings.TrimSpace(job.output) != "" {
		text += "\n\nResult\n" + job.output
	}
	return text + "\n\nR Run again   ·   Esc Back"
}
func (m *Model) operationMaxScroll() int {
	r := m.readingRect()
	return max(0, wrappedLines(m.operationText(), r.w)-r.h)
}
func (m *Model) operationView(f *frame, a appearance) {
	r := m.readingRect()
	f.richText(r, m.operationText(), a, min(m.screenScroll, m.operationMaxScroll()))
}

func (m *Model) quit() tea.Cmd {
	if m.job != nil && m.job.busy {
		m.job.cancel()
	}
	return tea.Quit
}
