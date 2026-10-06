package execution

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/LBYPatrick/ashley/internal/agents"
	"github.com/LBYPatrick/ashley/internal/history"
	"github.com/LBYPatrick/ashley/internal/sessions"
	"github.com/titanous/json5"
)

// trackConversation adds per-process callbacks without editing agent config files.
// Explicit IDs and resumed conversations need no discovery callbacks.
func trackConversation(cmd *exec.Cmd, job Job) (func(), error) {
	noop := func() {}
	if job.AgentSessionID != "" || (job.Agent != "codex" && job.Agent != "opencode" && job.Agent != "kilo") {
		return noop, nil
	}
	executable, err := os.Executable()
	if err != nil {
		return noop, err
	}
	callback := []string{executable, "__record-agent-session", job.HistoryPath, strconv.FormatInt(job.InvocationID, 10), job.Agent}
	if job.Agent == "codex" {
		// Keep the hook definition stable so Codex can retain its trust decision.
		// A private process is essential: a shared daemon can retain another run's
		// environment and incorrectly associate two concurrent invocations.
		command, _ := json.Marshal(sessions.Quote(executable) + " __record-agent-session-env")
		handler := fmt.Sprintf(`[{hooks=[{type="command",command=%s,timeout=5}]}]`, command)
		start := fmt.Sprintf(`hooks.SessionStart=[{matcher="^startup$",hooks=[{type="command",command=%s,timeout=5}]}]`, command)
		cmd.Args = append([]string{cmd.Args[0], "--no-daemon", "-c", start, "-c", "hooks.Stop=" + handler}, cmd.Args[1:]...)
		cmd.Env = append(cmd.Environ(), "ASHLEY_HISTORY_PATH="+job.HistoryPath, "ASHLEY_INVOCATION_ID="+strconv.FormatInt(job.InvocationID, 10))
		return noop, nil
	}
	dir, err := os.MkdirTemp("", "ashley-session-plugin-*")
	if err != nil {
		return noop, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	plugin := filepath.Join(dir, "tracking.mjs")
	if err := os.WriteFile(plugin, []byte(trackingPlugin(callback)), 0600); err != nil {
		cleanup()
		return noop, err
	}
	key := "OPENCODE_CONFIG_CONTENT"
	if job.Agent == "kilo" {
		key = "KILO_CONFIG_CONTENT"
	}
	config, err := withTrackingPlugin(os.Getenv(key), (&url.URL{Scheme: "file", Path: plugin}).String())
	if err != nil {
		cleanup()
		return noop, err
	}
	cmd.Env = append(cmd.Environ(), key+"="+config)
	return cleanup, nil
}

func trackingPlugin(callback []string) string {
	argv, _ := json.Marshal(callback)
	return `import {execFileSync} from "node:child_process";
export default async ({directory}) => {
  let linked = false;
  return {event: async ({event}) => {
    if (linked || event.type !== "session.created") return;
    const info = event.properties?.info;
    if (!info?.id || info.parentID || info.directory !== directory) return;
    const argv = ` + string(argv) + `;
    try {
      execFileSync(argv[0], argv.slice(1), {
        input: JSON.stringify({session_id: info.id, cwd: info.directory}),
        stdio: ["pipe", "ignore", "pipe"], timeout: 5000
      });
      linked = true;
    } catch (error) { console.error("Ashley conversation tracking failed:", error.message); }
  }};
};
`
}

func withTrackingPlugin(raw, plugin string) (string, error) {
	config := map[string]any{}
	if raw != "" {
		if err := json5.Unmarshal([]byte(raw), &config); err != nil {
			return "", fmt.Errorf("invalid inline agent config: %w", err)
		}
		if config == nil {
			return "", fmt.Errorf("inline agent config must be an object")
		}
	}
	var plugins []any
	if value, ok := config["plugin"]; ok {
		var valid bool
		plugins, valid = value.([]any)
		if !valid {
			return "", fmt.Errorf("inline agent plugin config must be an array")
		}
	}
	config["plugin"] = append(plugins, plugin)
	data, err := json.Marshal(config)
	return string(data), err
}

// RecordAgentSession is the private lifecycle callback used by agent processes.
// It accepts only an ID and directory, never arbitrary history fields or commands.
func RecordAgentSession(args []string, input io.Reader) error {
	if len(args) != 3 {
		return fmt.Errorf("session callback requires database, invocation ID, and agent")
	}
	id, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil || id <= 0 || !agents.Valid(args[2]) {
		return fmt.Errorf("invalid session callback target")
	}
	var event struct {
		SessionID string `json:"session_id"`
		CWD       string `json:"cwd"`
		Source    string `json:"source"`
	}
	data, err := io.ReadAll(io.LimitReader(input, (1<<20)+1))
	if err != nil {
		return err
	}
	if len(data) > 1<<20 {
		return fmt.Errorf("session callback too large")
	}
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}
	if !agents.ValidConversationID(event.SessionID) || !filepath.IsAbs(event.CWD) {
		return fmt.Errorf("invalid session callback payload")
	}
	if event.Source != "" && event.Source != "startup" {
		return nil
	}
	store, err := history.Open(args[0])
	if err != nil {
		return err
	}
	defer store.Close()
	row, err := store.Get(id)
	if err != nil {
		return err
	}
	if !sameDirectory(row.CWD, event.CWD) {
		return fmt.Errorf("session directory does not match invocation")
	}
	return store.LinkAgentSession(id, args[2], event.SessionID)
}

func sameDirectory(a, b string) bool {
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	x, err := filepath.EvalSymlinks(a)
	if err != nil {
		return false
	}
	y, err := filepath.EvalSymlinks(b)
	return err == nil && x == y
}
