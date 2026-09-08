package sessions

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDisplayLogRedraws(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{"\x1b[31mred\x1b[0m\r\nnext\n", "red\nnext"},
		{"progress 10%\r\x1b[2Kdone\n", "done"},
		{"abc\bD", "abD"},
		{"first\nold\x1b[1A\r\x1b[2Knew", "new\nold"},
		{"abc\rX\x1b[K", "X"},
		{"old\x1b[2J\x1b[Hnew", "new"},
		{"界界\rAB", "AB界"},
		{"a\tb", "a       b"},
		{"\x1b]0;malicious title\aokay\x1b[?1049h", "okay"},
	} {
		if got := DisplayLog(tc.raw); got != tc.want {
			t.Errorf("%q: got %q, want %q", tc.raw, got, tc.want)
		}
	}
}
func TestPreviewUsesRenderedTmuxAndFallsBackToLog(t *testing.T) {
	s := Session{TmuxSession: "ash-test", LogFile: filepath.Join(t.TempDir(), "log")}
	os.WriteFile(s.LogFile, []byte("progress\r\x1b[2Kcomplete\n"), 0600)
	m := Manager{Run: func(args []string, _ string) (string, error) {
		if !reflect.DeepEqual(args, []string{"capture-pane", "-p", "-t", "ash-test", "-S", "-50"}) {
			t.Fatal(args)
		}
		return "old\nrendered\nfinal\n\n", nil
	}}
	if got, err := m.Preview(s, 2, true); err != nil || got != "rendered\nfinal" {
		t.Fatal(got, err)
	}
	m.Run = func([]string, string) (string, error) { return "", errors.New("session exited") }
	if got, err := m.Preview(s, 50, true); err != nil || got != "complete" {
		t.Fatal(got, err)
	}
	if got, err := m.Preview(s, 50, false); err != nil || strings.ContainsAny(got, "\x1b\r") {
		t.Fatal(got, err)
	}
}
