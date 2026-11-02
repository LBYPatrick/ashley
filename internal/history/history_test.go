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

func TestDirectoryFiltersApplyToQueriesCountsAndStats(t *testing.T) {
	s := openTest(t)
	for _, row := range []Invocation{
		{Skill: "feat", AgentType: "codex", Question: "API changes", CWD: "/Work/Project_100%/api"},
		{Skill: "feat", AgentType: "claude", Question: "API changes", CWD: "/Work/Project_100%/web"},
		{Skill: "commit", AgentType: "codex", Question: "API changes", CWD: "/Work/Project_100%/api"},
		{Skill: "feat", AgentType: "codex", Question: "/Work/Project_100%/api", CWD: "/other"},
		{Skill: "feat", AgentType: "codex", CWD: "/Work/ProjectX100Y/api"},
	} {
		if _, err := s.Record(row); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name   string
		filter Filter
		want   int
	}{
		{"case insensitive", Filter{CWD: "/work/project_100%"}, 3},
		{"combined filters", Filter{CWD: "project_100%/API", Skill: "feat", Agent: "codex", Search: "changes"}, 1},
		{"literal underscore", Filter{CWD: "_"}, 3},
		{"literal percent", Filter{CWD: "%"}, 3},
		{"directory only", Filter{CWD: "/other"}, 1},
		{"no matches", Filter{CWD: "/missing"}, 0},
		{"blank", Filter{CWD: " \t"}, 5},
		{"generic search unchanged", Filter{Search: "project_100%"}, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := s.Query(tc.filter, -1, 0)
			if err != nil || len(rows) != tc.want {
				t.Fatalf("Query = %v, %v; want %d rows", rows, err, tc.want)
			}
			count, err := s.Count(tc.filter)
			if err != nil || count != tc.want {
				t.Fatalf("Count = %d, %v; want %d", count, err, tc.want)
			}
			stats, err := s.Stats(tc.filter)
			if err != nil || stats.Total != tc.want {
				t.Fatalf("Stats = %v, %v; want total %d", stats, err, tc.want)
			}
			skills, agents := 0, 0
			for _, usage := range stats.TopSkills {
				skills += usage.Count
			}
			for _, usage := range stats.ByAgent {
				agents += usage.Count
			}
			if skills != tc.want || agents != tc.want {
				t.Fatalf("Stats groups = %v; want %d entries", stats, tc.want)
			}
		})
	}
}

func TestDirectoryFilterUnicodeCase(t *testing.T) {
	s := openTest(t)
	if _, err := s.Record(Invocation{Skill: "feat", CWD: "/Work/ÜBER/Änderungen"}); err != nil {
		t.Fatal(err)
	}
	count, err := s.Count(Filter{CWD: "/über/änderungen"})
	if err != nil || count != 1 {
		t.Fatalf("Count = %d, %v; want 1", count, err)
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

func TestAgentSessionRoundTrip(t *testing.T) {
	s := openTest(t)
	id, err := s.Record(Invocation{Skill: "raw", AgentType: "grok", AgentSessionID: "conversation-one", SessionID: "deadbeef"})
	if err != nil {
		t.Fatal(err)
	}
	row, err := s.Get(id)
	if err != nil || row.AgentSessionID != "conversation-one" || row.SessionID != "deadbeef" {
		t.Fatal(row, err)
	}
	if _, err := s.Get(999); err == nil {
		t.Fatal("missing row found")
	}
	if _, err := s.Get(0); err == nil {
		t.Fatal("invalid row found")
	}
	if err := s.LinkAgentSession(id, "grok", "conversation-two"); err == nil {
		t.Fatal("replaced existing conversation")
	}
	rows, err := s.Query(Filter{Agent: "grok", AgentSessionID: "conversation-one"}, -1, 0)
	if err != nil || len(rows) != 1 {
		t.Fatal(rows, err)
	}
}

func TestSessionIDFilter(t *testing.T) {
	s := openTest(t)
	var ids []int64
	for _, row := range []Invocation{
		{SessionID: "session_100%", Skill: "feat", AgentType: "codex", CWD: "/work/api", Question: "First change", Timestamp: "2026-11-01T01:00:00Z"},
		{SessionID: "session_100%", Skill: "fix", AgentType: "claude", CWD: "/work/web", Question: "Second change", Timestamp: "2026-11-01T02:00:00Z"},
		{SessionID: "sessionX100other", Skill: "feat", AgentType: "codex", CWD: "/work/api", Timestamp: "2026-11-01T03:00:00Z"},
		{SessionID: "session_100%-child", Skill: "feat", AgentType: "codex", CWD: "/work/api", Timestamp: "2026-11-01T04:00:00Z"},
	} {
		id, err := s.Record(row)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	for _, tc := range []struct {
		name   string
		filter Filter
		limit  int
		want   []int64
	}{
		{"exact ID", Filter{SessionID: "session_100%"}, -1, []int64{ids[1], ids[0]}},
		{"newest match", Filter{SessionID: "session_100%"}, 1, []int64{ids[1]}},
		{"combined", Filter{SessionID: "session_100%", Skill: "feat", Agent: "codex", CWD: "/api", Search: "First"}, -1, []int64{ids[0]}},
		{"combined no match", Filter{SessionID: "session_100%", Agent: "codex", Skill: "fix"}, -1, []int64{}},
		{"missing", Filter{SessionID: "missing"}, -1, []int64{}},
		{"case sensitive", Filter{SessionID: "SESSION_100%"}, -1, []int64{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := s.Query(tc.filter, tc.limit, 0)
			if err != nil {
				t.Fatal(err)
			}
			got := []int64{}
			for _, row := range rows {
				got = append(got, row.ID)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Query = %v; want %v", got, tc.want)
			}
		})
	}
	count, err := s.Count(Filter{SessionID: "session_100%"})
	if err != nil || count != 2 {
		t.Fatalf("Count = %d, %v; want 2", count, err)
	}
	stats, err := s.Stats(Filter{SessionID: "session_100%"})
	if err != nil || stats.Total != 2 {
		t.Fatalf("Stats = %v, %v; want total 2", stats, err)
	}
}

func TestNameSurvivesReopenAndRenameIsScoped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.Record(Invocation{Name: "API work", Skill: "raw", SessionID: "abc"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.Record(Invocation{Name: "Other task", Skill: "raw"})
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	v, err := s.Get(id)
	if err != nil || v.Name != "API work" {
		t.Fatal(v, err)
	}
	if err = s.Rename(id, "abc", "API release"); err != nil {
		t.Fatal(err)
	}
	v, _ = s.Get(other)
	if v.Name != "Other task" {
		t.Fatal("unrelated name changed", v)
	}
}
