//go:build integration && (darwin || linux)

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/LBYPatrick/ashley/internal/history"
)

func TestNativeConversationCallback(t *testing.T) {
	s := newSandbox(t)
	store, err := history.Open(historyPath(s))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, agent := range []string{"codex", "opencode", "kilo"} {
		id, err := store.Record(history.Invocation{Skill: "raw", CWD: s.home, AgentType: agent})
		if err != nil {
			t.Fatal(err)
		}
		payload, _ := json.Marshal(map[string]string{"session_id": "session-" + agent, "cwd": s.home})
		args := []string{"__record-agent-session", historyPath(s), fmt.Sprint(id), agent}
		if agent == "codex" {
			args = []string{"__record-agent-session-env"}
			s.env["ASHLEY_HISTORY_PATH"] = historyPath(s)
			s.env["ASHLEY_INVOCATION_ID"] = fmt.Sprint(id)
		}
		cmd := s.command(context.Background(), binary, args...)
		cmd.Stdin = strings.NewReader(string(payload))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatal(err, string(out))
		}
		row, err := store.Get(id)
		if err != nil || row.AgentSessionID != "session-"+agent {
			t.Fatal(row, err)
		}
	}
	output := s.must(binary, "history", "show", "--json")
	if !strings.Contains(output, `"agent_session_id":"session-codex"`) {
		t.Fatal(output)
	}
}
