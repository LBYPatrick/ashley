// Package history stores invocation records in Ashley's existing SQLite database.
package history

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Path returns the legacy database location for the given platform.
func Path(home, platform, xdg string) string {
	switch platform {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "ashley", "history.db")
	case "linux":
		if xdg == "" {
			xdg = filepath.Join(home, ".local", "share")
		}
		return filepath.Join(xdg, "ashley", "history.db")
	default:
		return filepath.Join(home, ".ashley", "history.db")
	}
}

// Store is a concurrency-safe database handle; callers must close it.
type Store struct{ db *sql.DB }

const schema = `CREATE TABLE IF NOT EXISTS invocations (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 timestamp TEXT NOT NULL,
 skill TEXT NOT NULL,
 question TEXT NOT NULL DEFAULT '',
 cwd TEXT NOT NULL DEFAULT '',
 permission TEXT NOT NULL DEFAULT 'default',
 detached INTEGER NOT NULL DEFAULT 0,
 session_id TEXT NOT NULL DEFAULT '',
 exit_code INTEGER DEFAULT NULL,
 duration_s REAL DEFAULT NULL,
 outcome TEXT NOT NULL DEFAULT 'unknown',
 agent_type TEXT NOT NULL DEFAULT 'claude'
);
CREATE INDEX IF NOT EXISTS idx_invocations_timestamp ON invocations(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_invocations_skill ON invocations(skill);`

// Open creates or migrates the existing database without changing its location or rows.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.initialize(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) initialize() error {
	if _, err := s.db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		return err
	}
	// An immediate transaction serializes schema checks across Ashley processes.
	conn, err := s.db.Conn(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), "BEGIN IMMEDIATE"); err != nil {
		return err
	}
	defer conn.ExecContext(context.Background(), "ROLLBACK")
	if _, err := conn.ExecContext(context.Background(), schema); err != nil {
		return err
	}
	rows, err := conn.QueryContext(context.Background(), "PRAGMA table_info(invocations)")
	if err != nil {
		return err
	}
	existing := map[string]bool{}
	for rows.Next() {
		var cid, notnull, pk int
		var name, kind string
		var value any
		if err := rows.Scan(&cid, &name, &kind, &notnull, &value, &pk); err != nil {
			rows.Close()
			return err
		}
		existing[name] = true
	}
	err = errors.Join(rows.Err(), rows.Close())
	if err != nil {
		return err
	}
	for _, column := range [][2]string{{"exit_code", "INTEGER DEFAULT NULL"}, {"duration_s", "REAL DEFAULT NULL"}, {"outcome", "TEXT NOT NULL DEFAULT 'unknown'"}, {"agent_type", "TEXT NOT NULL DEFAULT 'claude'"}} {
		if !existing[column[0]] {
			if _, err := conn.ExecContext(context.Background(), "ALTER TABLE invocations ADD COLUMN "+column[0]+" "+column[1]); err != nil {
				return err
			}
		}
	}
	_, err = conn.ExecContext(context.Background(), "COMMIT")
	return err
}

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

// Invocation mirrors legacy history rows, including unknown outcomes.
type Invocation struct {
	ID         int64    `json:"id"`
	Timestamp  string   `json:"timestamp"`
	Skill      string   `json:"skill"`
	Question   string   `json:"question"`
	CWD        string   `json:"cwd"`
	Permission string   `json:"permission"`
	Detached   bool     `json:"detached"`
	SessionID  string   `json:"session_id"`
	ExitCode   *int     `json:"exit_code"`
	DurationS  *float64 `json:"duration_s"`
	Outcome    string   `json:"outcome"`
	AgentType  string   `json:"agent_type"`
}

// Record inserts a new invocation and returns its ID.
func (s *Store) Record(v Invocation) (int64, error) {
	if v.Timestamp == "" {
		v.Timestamp = stamp(time.Now())
	}
	if v.Permission == "" {
		v.Permission = "default"
	}
	if v.AgentType == "" {
		v.AgentType = "claude"
	}
	result, err := s.db.Exec(`INSERT INTO invocations (timestamp,skill,question,cwd,permission,detached,session_id,agent_type) VALUES (?,?,?,?,?,?,?,?)`, v.Timestamp, v.Skill, v.Question, v.CWD, v.Permission, v.Detached, v.SessionID, v.AgentType)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}
func stamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000000+00:00") }

// Filter selects records by skill, coding agent, or a SQL LIKE substring search.
type Filter struct{ Skill, Agent, Search string }

func (f Filter) where() (string, []any) {
	var conditions []string
	var args []any
	if f.Skill != "" {
		conditions = append(conditions, "skill = ?")
		args = append(args, f.Skill)
	}
	if f.Agent != "" {
		conditions = append(conditions, "agent_type = ?")
		args = append(args, f.Agent)
	}
	if f.Search != "" {
		conditions = append(conditions, "(question LIKE ? OR cwd LIKE ? OR skill LIKE ?)")
		p := "%" + f.Search + "%"
		args = append(args, p, p, p)
	}
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

// Query returns newest records first; limit -1 retrieves all records.
func (s *Store) Query(filter Filter, limit, offset int) ([]Invocation, error) {
	where, args := filter.where()
	args = append(args, limit, offset)
	rows, err := s.db.Query(`SELECT id,timestamp,skill,question,cwd,permission,detached,session_id,exit_code,duration_s,COALESCE(NULLIF(outcome,''),'unknown'),COALESCE(NULLIF(agent_type,''),'claude') FROM invocations`+where+` ORDER BY timestamp DESC,id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Invocation{}
	for rows.Next() {
		var v Invocation
		if err := rows.Scan(&v.ID, &v.Timestamp, &v.Skill, &v.Question, &v.CWD, &v.Permission, &v.Detached, &v.SessionID, &v.ExitCode, &v.DurationS, &v.Outcome, &v.AgentType); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}

// Count counts matching rows independently of pagination.
func (s *Store) Count(filter Filter) (int, error) {
	where, args := filter.where()
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM invocations"+where, args...).Scan(&count)
	return count, err
}

// RecordOutcome updates completion status while preserving original invocation data.
func (s *Store) RecordOutcome(id int64, code int, duration float64) error {
	outcome := "success"
	if code != 0 {
		outcome = "failure"
	}
	_, err := s.db.Exec("UPDATE invocations SET exit_code=?,duration_s=?,outcome=? WHERE id=?", code, duration, outcome, id)
	return err
}
func (s *Store) remove(query string, args ...any) (int64, error) {
	result, err := s.db.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Prune deletes records older than the specified number of days.
func (s *Store) Prune(days int) (int64, error) {
	if days < 0 {
		return 0, fmt.Errorf("days must be nonnegative")
	}
	return s.remove("DELETE FROM invocations WHERE timestamp < ?", stamp(time.Now().AddDate(0, 0, -days)))
}

// Clear removes all history records.
func (s *Store) Clear() (int64, error) { return s.remove("DELETE FROM invocations") }

// Delete removes one invocation.
func (s *Store) Delete(id int64) (int64, error) {
	return s.remove("DELETE FROM invocations WHERE id=?", id)
}

// Usage is a grouped invocation count.
type Usage struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Stats contains total usage and skill/agent breakdowns.
type Stats struct {
	Total     int     `json:"total"`
	TopSkills []Usage `json:"top_skills"`
	ByAgent   []Usage `json:"by_agent"`
}

// Stats returns deterministic usage rankings, optionally filtered.
func (s *Store) Stats(f Filter) (Stats, error) {
	total, err := s.Count(f)
	if err != nil {
		return Stats{}, err
	}
	result := Stats{Total: total, TopSkills: []Usage{}, ByAgent: []Usage{}}
	where, args := f.where()
	for _, group := range []struct {
		column string
		limit  int
		dest   *[]Usage
	}{{"skill", 10, &result.TopSkills}, {"agent_type", -1, &result.ByAgent}} {
		rows, err := s.db.Query("SELECT "+group.column+",COUNT(*) AS n FROM invocations"+where+" GROUP BY "+group.column+" ORDER BY n DESC,"+group.column+" ASC LIMIT ?", append(append([]any{}, args...), group.limit)...)
		if err != nil {
			return Stats{}, err
		}
		for rows.Next() {
			var name sql.NullString
			var count int
			if err := rows.Scan(&name, &count); err != nil {
				rows.Close()
				return Stats{}, err
			}
			key := name.String
			if group.column == "agent_type" && key == "" {
				key = "claude"
			}
			*group.dest = append(*group.dest, Usage{key, count})
		}
		err = errors.Join(rows.Err(), rows.Close())
		if err != nil {
			return Stats{}, err
		}
	}
	return result, nil
}

// TimeDisplay renders a timestamp without changing its stored timezone.
func (v Invocation) TimeDisplay() string {
	if parsed, err := time.Parse(time.RFC3339Nano, v.Timestamp); err == nil {
		return parsed.Format("2006-01-02 15:04:05")
	}
	runes := []rune(v.Timestamp)
	return string(runes[:min(19, len(runes))])
}

// QuestionShort limits history rows to 80 characters.
func (v Invocation) QuestionShort() string {
	if v.Question == "" {
		return "(no question)"
	}
	runes := []rune(v.Question)
	if len(runes) > 80 {
		return string(runes[:80]) + "..."
	}
	return v.Question
}

// DurationDisplay formats an optional execution duration.
func (v Invocation) DurationDisplay() string {
	if v.DurationS == nil {
		return "—"
	}
	secs := int(*v.DurationS)
	if secs < 60 {
		return fmt.Sprintf("%ds", secs)
	}
	if secs < 3600 {
		return fmt.Sprintf("%dm %ds", secs/60, secs%60)
	}
	return fmt.Sprintf("%dh %dm", secs/3600, (secs%3600)/60)
}

// OutcomeIcon returns the history browser's status symbol.
func (v Invocation) OutcomeIcon() string {
	switch v.Outcome {
	case "success":
		return "✓"
	case "failure":
		return "✗"
	case "cancelled":
		return "○"
	default:
		return "?"
	}
}
