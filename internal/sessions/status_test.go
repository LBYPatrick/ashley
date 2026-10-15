package sessions

import (
	"errors"
	"reflect"
	"testing"
)

func TestAliveAllBatchesBackends(t *testing.T) {
	for _, failed := range []bool{false, true} {
		calls := 0
		runner := func(args []string, _ string) (string, error) {
			calls++
			if args[0] != "list-sessions" {
				t.Fatalf("per-session command: %v", args)
			}
			if failed {
				return "", errors.New("no server")
			}
			return "live\n live-longer \n", nil
		}
		manager := Manager{Run: runner, ZellijRun: runner}
		input := []Session{
			{ID: "legacy", TmuxSession: "live"},
			{ID: "tmux", Backend: "tmux", TmuxSession: "live-longer"},
			{ID: "zellij", Backend: "zellij", ZellijSession: "live"},
			{ID: "finished", Backend: "zellij", ZellijSession: "liv"},
		}
		want := map[string]bool{"legacy": !failed, "tmux": !failed, "zellij": !failed, "finished": false}
		if got := manager.AliveAll(input); !reflect.DeepEqual(got, want) || calls != 2 {
			t.Fatalf("failed=%v: states=%v calls=%d", failed, got, calls)
		}
		calls = 0
		if got := manager.AliveAll(input[:2]); len(got) != 2 || calls != 1 {
			t.Fatalf("queried an unused backend: states=%v calls=%d", got, calls)
		}
		calls = 0
		if got := manager.AliveAll(nil); len(got) != 0 || calls != 0 {
			t.Fatal("empty list queried a backend")
		}
	}
}
