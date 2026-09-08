package sessions

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Session preserves metadata written by the Python session manager.
type Session struct {
	ID             string   `json:"id"`
	Skill          string   `json:"skill"`
	Question       string   `json:"question"`
	TmuxSession    string   `json:"tmux_session"`
	LogFile        string   `json:"log_file"`
	StartedAt      string   `json:"started_at"`
	CWD            string   `json:"cwd"`
	PermissionMode string   `json:"permission_mode"`
	Agent          string   `json:"agent"`
	ExtraFlags     []string `json:"extra_flags"`
}

// Manager locates persistent metadata and executes tmux commands.
type Manager struct {
	Dir string
	Run Commander
}

func (m Manager) command(args ...string) (string, error) {
	run := m.Run
	if run == nil {
		run = Tmux
	}
	return run(args, "")
}

// User returns the current user's session manager.
func User() (Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Manager{}, err
	}
	return Manager{Dir: filepath.Join(home, ".ashley", "sessions"), Run: Tmux}, nil
}
func validID(id string) bool {
	return id != "" && id != "." && filepath.Base(id) == id && !strings.ContainsAny(id, "/\\")
}

// Save atomically persists a session without depending on a source checkout.
func (m Manager) Save(s Session) error {
	if !validID(s.ID) {
		return fmt.Errorf("invalid session ID: %s", s.ID)
	}
	if err := os.MkdirAll(m.Dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(m.Dir, ".session-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, writeErr := f.Write(append(data, '\n'))
	closeErr := f.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(m.Dir, s.ID+".json"))
}

// Load reads a session by its complete ID.
func (m Manager) Load(id string) (Session, error) {
	if !validID(id) {
		return Session{}, fmt.Errorf("invalid session ID: %s", id)
	}
	data, err := os.ReadFile(filepath.Join(m.Dir, id+".json"))
	if err != nil {
		return Session{}, err
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return Session{}, err
	}
	if s.ID != id || s.TmuxSession == "" || s.LogFile == "" {
		return Session{}, fmt.Errorf("invalid session metadata: %s", id)
	}
	return s, nil
}

// All skips corrupt records and returns newest sessions first.
func (m Manager) All() ([]Session, error) {
	entries, err := os.ReadDir(m.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return []Session{}, nil
	}
	if err != nil {
		return nil, err
	}
	result := []Session{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		s, err := m.Load(strings.TrimSuffix(entry.Name(), ".json"))
		if err == nil {
			result = append(result, s)
		}
	}
	return Sort(result, "time"), nil
}

// Sort returns a copy sorted by time or skill, retaining newest-first skill groups.
func Sort(input []Session, mode string) []Session {
	result := append([]Session{}, input...)
	sort.SliceStable(result, func(i, j int) bool { return result[i].StartedAt > result[j].StartedAt })
	if mode == "skill" {
		sort.SliceStable(result, func(i, j int) bool { return strings.ToLower(result[i].Skill) < strings.ToLower(result[j].Skill) })
	}
	return result
}

// Resolve accepts an unambiguous ID prefix.
func (m Manager) Resolve(prefix string) (Session, error) {
	if !validID(prefix) {
		return Session{}, fmt.Errorf("invalid session ID: %s", prefix)
	}
	all, err := m.All()
	if err != nil {
		return Session{}, err
	}
	var matches []Session
	for _, s := range all {
		if s.ID == prefix {
			return s, nil
		}
		if strings.HasPrefix(s.ID, prefix) {
			matches = append(matches, s)
		}
	}
	if len(matches) == 0 {
		return Session{}, fmt.Errorf("session not found: %s", prefix)
	}
	if len(matches) > 1 {
		return Session{}, fmt.Errorf("ambiguous session ID: %s", prefix)
	}
	return matches[0], nil
}

// Alive reports whether the tmux session still exists.
func (m Manager) Alive(s Session) bool {
	_, err := m.command("has-session", "-t", s.TmuxSession)
	return err == nil
}

// Remove deletes metadata and optionally its log.
func (m Manager) Remove(s Session, log bool) error {
	if !validID(s.ID) {
		return fmt.Errorf("invalid session ID: %s", s.ID)
	}
	err := os.Remove(filepath.Join(m.Dir, s.ID+".json"))
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	if log {
		e := os.Remove(s.LogFile)
		if !errors.Is(e, os.ErrNotExist) {
			err = errors.Join(err, e)
		}
	}
	return err
}

// Kill terminates a running session and retains the log for later inspection.
func (m Manager) Kill(s Session) error {
	if m.Alive(s) {
		if _, err := m.command("kill-session", "-t", s.TmuxSession); err != nil {
			return err
		}
	}
	return m.Remove(s, false)
}

// KillAll terminates registered sessions, returning the live-session count.
func (m Manager) KillAll() (int, error) {
	all, err := m.All()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, s := range all {
		alive := m.Alive(s)
		if err := m.Kill(s); err != nil {
			return count, err
		}
		if alive {
			count++
		}
	}
	return count, nil
}

// CleanDead removes stale metadata and retains logs.
func (m Manager) CleanDead() (int, error) {
	all, err := m.All()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, s := range all {
		if !m.Alive(s) {
			if err := m.Remove(s, false); err != nil {
				return count, err
			}
			count++
		}
	}
	return count, nil
}

// ReadLog returns the last n lines, or the complete log when n is nonpositive.
func ReadLog(s Session, n int) (string, error) {
	data, err := os.ReadFile(s.LogFile)
	if errors.Is(err, os.ErrNotExist) {
		return "(no log file)", nil
	}
	if err != nil {
		return "", err
	}
	text := strings.ToValidUTF8(string(data), "�")
	if n <= 0 {
		return text, nil
	}
	lines := strings.SplitAfter(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, ""), nil
}

// Elapsed formats elapsed time since creation, including older naive timestamps.
func (s Session) Elapsed(now time.Time) string {
	started, err := time.Parse(time.RFC3339Nano, s.StartedAt)
	if err != nil {
		started, err = time.Parse("2006-01-02T15:04:05.999999", s.StartedAt)
	}
	if err != nil {
		return "?"
	}
	seconds := int(now.Sub(started).Seconds())
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%dm %ds", seconds/60, seconds%60)
	}
	return fmt.Sprintf("%dh %dm", seconds/3600, (seconds%3600)/60)
}

// Quote protects literal text in a POSIX shell command.
func Quote(text string) string { return "'" + strings.ReplaceAll(text, "'", "'\\''") + "'" }

const PromptFileMarker = "__ASHLEY_PROMPT_FILE__="

// ShellCommand expands spilled prompt arguments and cleans them on exit.
func ShellCommand(args []string, cwd string) (string, []string) {
	parts := make([]string, len(args))
	var files []string
	for i, arg := range args {
		if strings.HasPrefix(arg, PromptFileMarker) {
			path := strings.TrimPrefix(arg, PromptFileMarker)
			files = append(files, path)
			parts[i] = `"$(cat ` + Quote(path) + `)"`
		} else {
			parts[i] = Quote(arg)
		}
	}
	cleanup := ""
	for _, path := range files {
		cleanup += "rm -f " + Quote(path) + "; "
	}
	return "cd " + Quote(cwd) + " && " + strings.Join(parts, " ") + "; " + cleanup + "echo ''; echo '[Session ended - press Enter to close]'; read", files
}

// Create launches an agent and configures log capture and session-scoped scrolling.
func (m Manager) Create(s Session, args []string) (Session, error) {
	if len(args) == 0 {
		return Session{}, fmt.Errorf("agent command is empty")
	}
	prepared, err := m.Prepare(s)
	if err != nil {
		return Session{}, err
	}
	return m.Start(prepared, args)
}

// Prepare allocates metadata before history and supervisor files are written.
func (m Manager) Prepare(s Session) (Session, error) {
	id := make([]byte, 4)
	if _, err := rand.Read(id); err != nil {
		return Session{}, err
	}
	s.ID = hex.EncodeToString(id)
	s.TmuxSession = "ashley-" + s.ID
	s.LogFile = filepath.Join(m.Dir, s.ID+".log")
	s.StartedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return s, nil
}

// Start launches a prepared session after callers finish recording its context.
func (m Manager) Start(s Session, args []string) (Session, error) {
	if !validID(s.ID) || s.TmuxSession != "ashley-"+s.ID || len(args) == 0 {
		return Session{}, fmt.Errorf("invalid prepared session")
	}
	if err := os.MkdirAll(m.Dir, 0700); err != nil {
		return Session{}, err
	}
	command, _ := ShellCommand(args, s.CWD)
	// Start a waiting shell, configure logging, then release the agent. This
	// prevents fast startup output from being lost before pipe-pane attaches.
	gate := "ashley-start-" + s.ID
	command = "tmux wait-for " + Quote(gate) + "; " + command
	if _, err := m.command("new-session", "-d", "-s", s.TmuxSession, "-x", "200", "-y", "50", "bash", "-c", command); err != nil {
		return Session{}, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			m.command("kill-session", "-t", s.TmuxSession)
			m.Remove(s, true)
		}
	}()
	run := m.Run
	if run == nil {
		run = Tmux
	}
	if err := ConfigureScrolling(run, s.TmuxSession); err != nil {
		return Session{}, err
	}
	if _, err := m.command("pipe-pane", "-t", s.TmuxSession, "-o", "cat >> "+Quote(s.LogFile)); err != nil {
		return Session{}, err
	}
	if err := m.Save(s); err != nil {
		return Session{}, err
	}
	if _, err := m.command("wait-for", "-S", gate); err != nil {
		return Session{}, err
	}
	cleanup = false
	return s, nil
}
