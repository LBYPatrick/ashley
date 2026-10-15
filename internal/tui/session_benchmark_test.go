package tui

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/LBYPatrick/ashley/internal/sessions"
)

// Measure UI work with a slow multiplexer, excluding background command time.
func BenchmarkSessionNavigation(b *testing.B) {
	manager := sessions.Manager{Run: func([]string, string) (string, error) {
		time.Sleep(5 * time.Millisecond)
		return "Agent output", nil
	}}
	m := &Model{
		options: Options{Sessions: &manager}, screen: "sessions", width: 100, height: 30,
		sessionRows:  []sessions.Session{{ID: "bench", TmuxSession: "ashley-bench"}},
		sessionAlive: map[string]bool{"bench": true},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.updatePreview()
	}
}

// Compare the former per-session checks with the batched backend query.
func BenchmarkSessionStatusChecks(b *testing.B) {
	var rows []sessions.Session
	var names []string
	for i := range 32 {
		name := "ashley-" + strconv.Itoa(i)
		rows = append(rows, sessions.Session{ID: name, Backend: "zellij", ZellijSession: name})
		names = append(names, name)
	}
	output := strings.Join(names, "\n")
	manager := sessions.Manager{ZellijRun: func([]string, string) (string, error) {
		time.Sleep(time.Millisecond)
		return output, nil
	}}
	b.Run("per-session", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, row := range rows {
				manager.Alive(row)
			}
		}
	})
	b.Run("batched", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			manager.AliveAll(rows)
		}
	})
}
