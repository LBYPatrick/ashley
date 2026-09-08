package history

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func TestLegacyMigrationPreservesRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE invocations (id INTEGER PRIMARY KEY AUTOINCREMENT,timestamp TEXT NOT NULL,skill TEXT NOT NULL,question TEXT NOT NULL DEFAULT '',cwd TEXT NOT NULL DEFAULT '',permission TEXT NOT NULL DEFAULT 'default',detached INTEGER NOT NULL DEFAULT 0,session_id TEXT NOT NULL DEFAULT ''); INSERT INTO invocations(timestamp,skill,question) VALUES('2024-01-01T12:00:00+00:00','feat','legacy question')`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	for i := 0; i < 2; i++ {
		s, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		entries, err := s.Query(Filter{}, 50, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].Question != "legacy question" || entries[0].AgentType != "claude" || entries[0].Outcome != "unknown" || entries[0].ExitCode != nil {
			t.Fatal(entries)
		}
		s.Close()
	}
}
func TestQueriesAnalyticsAndOutcomes(t *testing.T) {
	s := openTest(t)
	var ids []int64
	for i, a := range []string{"claude", "codex", "grok", "opencode", "kilo", "codex"} {
		skill := "feat"
		if i%2 == 1 {
			skill = "commit"
		}
		id, err := s.Record(Invocation{Skill: skill, AgentType: a, Question: fmt.Sprintf("question %d", i), CWD: "/project", Timestamp: stamp(time.Now().Add(time.Duration(i) * time.Second)), Detached: i == 0})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := s.RecordOutcome(ids[0], 0, 72.5); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordOutcome(ids[1], 2, 1); err != nil {
		t.Fatal(err)
	}
	all, err := s.Query(Filter{}, -1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 6 || all[5].Outcome != "success" || *all[5].DurationS != 72.5 || !all[5].Detached || all[4].Outcome != "failure" {
		t.Fatal(all)
	}
	for _, c := range []struct {
		filter Filter
		count  int
	}{{Filter{Skill: "feat"}, 3}, {Filter{Agent: "codex"}, 2}, {Filter{Search: "question 3"}, 1}, {Filter{Search: "/project"}, 6}, {Filter{Search: "commit"}, 3}, {Filter{Skill: "' OR 1=1 --"}, 0}} {
		count, err := s.Count(c.filter)
		if err != nil || count != c.count {
			t.Fatal(count, err)
		}
		entries, err := s.Query(c.filter, -1, 0)
		if err != nil || len(entries) != c.count {
			t.Fatal(entries, err)
		}
	}
	page, err := s.Query(Filter{}, 2, 2)
	if err != nil || page[0].ID != ids[3] || page[1].ID != ids[2] {
		t.Fatal(page, err)
	}
	stats, err := s.Stats(Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Total != 6 || !reflect.DeepEqual(stats.TopSkills, []Usage{{"commit", 3}, {"feat", 3}}) || stats.ByAgent[0] != (Usage{"codex", 2}) {
		t.Fatal(stats)
	}
	filtered, err := s.Stats(Filter{Agent: "codex"})
	if err != nil || filtered.Total != 2 || len(filtered.ByAgent) != 1 {
		t.Fatal(filtered, err)
	}
	if n, err := s.Delete(ids[0]); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	if n, err := s.Clear(); err != nil || n != 5 {
		t.Fatal(n, err)
	}
	stats, err = s.Stats(Filter{})
	if err != nil || stats.Total != 0 || len(stats.ByAgent) != 0 {
		t.Fatal(stats, err)
	}
}
func TestPruneAndDefaultFields(t *testing.T) {
	s := openTest(t)
	if _, err := s.Record(Invocation{Skill: "old", Timestamp: "2000-01-01T00:00:00+00:00"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Record(Invocation{Skill: "new"}); err != nil {
		t.Fatal(err)
	}
	if n, err := s.Prune(30); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	entries, err := s.Query(Filter{}, 50, 0)
	if err != nil || len(entries) != 1 || entries[0].Permission != "default" || entries[0].AgentType != "claude" {
		t.Fatal(entries, err)
	}
	if _, err := s.Prune(-1); err == nil {
		t.Fatal("negative prune accepted")
	}
}
func TestConcurrentMigrationAndWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "concurrent.db")
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Go(func() {
			s, err := Open(path)
			if err != nil {
				t.Error(err)
				return
			}
			defer s.Close()
			if _, err := s.Record(Invocation{Skill: "feat"}); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if n, err := s.Count(Filter{}); err != nil || n != 5 {
		t.Fatal(n, err)
	}
}
func TestPaths(t *testing.T) {
	for _, c := range []struct{ os, xdg, want string }{{"darwin", "/ignored", "/home/u/Library/Application Support/ashley/history.db"}, {"linux", "", "/home/u/.local/share/ashley/history.db"}, {"linux", "/data", "/data/ashley/history.db"}, {"other", "", "/home/u/.ashley/history.db"}} {
		if got := Path("/home/u", c.os, c.xdg); got != c.want {
			t.Fatal(got)
		}
	}
}
