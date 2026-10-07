package cli

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"

	"github.com/LBYPatrick/ashley/internal/agents"
	"github.com/LBYPatrick/ashley/internal/history"
	"github.com/LBYPatrick/ashley/internal/invocation"
	"github.com/LBYPatrick/ashley/internal/sessions"
	"github.com/LBYPatrick/ashley/internal/skills"
)

func resumeHistory(args []string, catalog skills.Catalog, stdout, stderr io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: ash history resume <history-id>")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil || id <= 0 {
		return fmt.Errorf("history ID must be a positive integer")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	store, err := history.Open(history.Path(home, runtime.GOOS, os.Getenv("XDG_DATA_HOME")))
	if err != nil {
		return err
	}
	defer store.Close()
	entry, err := store.Get(id)
	if err != nil {
		return err
	}
	manager, err := sessions.User()
	if err != nil {
		return err
	}
	// Also find a resumed instance when the user selects the original row again.
	rows := []history.Invocation{entry}
	if entry.AgentSessionID != "" {
		rows, err = store.Query(history.Filter{Agent: entry.AgentType, AgentSessionID: entry.AgentSessionID}, -1, 0)
		if err != nil {
			return err
		}
	}
	for _, row := range rows {
		if row.SessionID == "" {
			continue
		}
		s, err := manager.Load(row.SessionID)
		if err == nil && manager.Alive(s) {
			return attachSession(manager, s, stdout, stderr)
		}
	}
	if entry.AgentSessionID == "" {
		return fmt.Errorf("history entry %d has no agent conversation ID; old runs and sessions with disabled tracking cannot be resumed automatically", id)
	}
	if !agents.Valid(entry.AgentType) {
		return fmt.Errorf("unsupported coding agent: %s", entry.AgentType)
	}
	info, err := os.Stat(entry.CWD)
	if err != nil {
		return fmt.Errorf("original working directory unavailable: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("original working directory is not a directory: %s", entry.CWD)
	}
	o := invocation.Options{Name: entry.Name, Agent: entry.AgentType, Skill: entry.Skill, Question: entry.Question, ResumeID: entry.AgentSessionID, WorkDir: entry.CWD}
	switch entry.Permission {
	case "default", "":
		o.Normal = true
	case "dsp":
		o.DSP = true
	case "afk":
		o.AFK = true
	case "auto":
		o.Auto = true
	default:
		return fmt.Errorf("unknown recorded permission mode: %s", entry.Permission)
	}
	return launch("run", o, false, catalog, stdout, stderr)
}
