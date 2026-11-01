//go:build integration && (darwin || linux)

package integration

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LBYPatrick/ashley/internal/history"
	"github.com/LBYPatrick/ashley/internal/sessions"
	"github.com/charmbracelet/x/ansi"
)

func TestDirectoryFiltersInStandaloneBinary(t *testing.T) {
	s := newSandbox(t)
	s.env["NO_COLOR"] = "1"
	store, err := history.Open(historyPath(s))
	if err != nil {
		t.Fatal(err)
	}
	manager := sessions.Manager{Dir: filepath.Join(s.home, ".ashley", "sessions")}
	for i, path := range []string{"/work/Client_100%/Billing API", "/work/ClientX100Z/Billing API"} {
		id := []string{"literal-path", "similar-path"}[i]
		if _, err := store.Record(history.Invocation{Skill: "coding", Name: "Invoice review", CWD: path, AgentType: "codex"}); err != nil {
			t.Fatal(err)
		}
		if err := manager.Save(sessions.Session{ID: id, Skill: "coding", Name: "Invoice review", CWD: path,
			Agent: "codex", TmuxSession: "ashley-" + id, LogFile: filepath.Join(s.home, id+".log")}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		args []string
		want int
	}{
		{"history directory", []string{"history", "show", "--json", "--cwd", "CLIENT_100%"}, 1},
		{"history combined", []string{"history", "show", "--json", "--cwd", "Billing API", "--search", "invoice", "--agent", "codex"}, 2},
		{"history empty", []string{"history", "show", "--json", "--cwd", "missing"}, 0},
		{"sessions directory", []string{"sessions", "--json", "--cwd=CLIENT_100%"}, 1},
		{"sessions combined", []string{"sessions", "--json", "--cwd", "Billing API", "--search", "invoice"}, 2},
		{"sessions empty", []string{"sessions", "--json", "--cwd", "missing"}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var rows []struct {
				CWD string `json:"cwd"`
			}
			out := s.must(binary, tc.args...)
			if err := json.Unmarshal([]byte(out), &rows); err != nil {
				t.Fatal(err, out)
			}
			if len(rows) != tc.want || (tc.want == 1 && rows[0].CWD != "/work/Client_100%/Billing API") {
				t.Fatal(rows, out)
			}
		})
	}
	var stats history.Stats
	out := s.must(binary, "history", "stats", "--json", "--cwd", "CLIENT_100%")
	if err := json.Unmarshal([]byte(out), &stats); err != nil || stats.Total != 1 {
		t.Fatal(stats, err, out)
	}
	requireContains(t, s.must(binary, "sessions", "--list", "--cwd", "CLIENT_100%"), "/work/Client_100%/Billing API")
	t.Run("terminal directory scope", func(t *testing.T) {
		p := startTerminal(t, s, binary, "history", "browse")
		p.waitFor("Search history")
		p.send("/")
		p.until(func() bool { return strings.Contains(ansi.Strip(p.text()), "enter Results") })
		offset := len(p.text())
		p.send("Invoice")
		p.until(func() bool { return strings.Contains(p.text()[offset:], "Invoice") })
		p.send("\x06") // Ctrl+F restricts the same query to working directories.
		p.waitFor("No matching results.")
		offset = len(p.text())
		p.send("\x15") // Ctrl+U clears the query without leaving the search field.
		p.until(func() bool { return strings.Contains(p.text()[offset:], "Invoice review") })
		p.quit()
	})
}
