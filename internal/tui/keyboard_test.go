package tui

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
)

var _ term.File = (*keyboardOutput)(nil)

func TestExtendedTabKeys(t *testing.T) {
	for n := 1; n <= 5; n++ {
		for _, seq := range []string{fmt.Sprintf("\x1b[%d;5u", 48+n), fmt.Sprintf("\x1b[27;5;%d~", 48+n), fmt.Sprintf("\x1b[%d;5:2u", 48+n)} {
			got := decodeExtendedKey(seq, nil)
			if got != tabShortcut(n) {
				t.Fatal(seq, got)
			}
		}
	}
	for _, seq := range []string{"\x1b[49;5:3u", "\x1b[49;9u", "\x1b[bad", "\x1b[27;5~"} {
		if decodeExtendedKey(seq, nil) != nil {
			t.Fatal(seq)
		}
	}
	for seq, want := range map[string]string{"\x1b[27u": "esc", "\x1b[110;5u": "ctrl+n", "\x1b[27;5;112~": "ctrl+p", "\x1b[9;2u": "shift+tab", "\x1b[13u": "enter"} {
		key, ok := decodeExtendedKey(seq, nil).(tea.KeyMsg)
		if !ok || key.String() != want {
			t.Fatal(seq, key)
		}
	}
}
func TestNavigationPreservesTextAndFitsMobile(t *testing.T) {
	m := newModel(t)
	for _, width := range []int{20, 32, 50, 80, 120} {
		m.width = width
		controls := m.navControls()
		if len(controls) != 5 {
			t.Fatal(width, controls)
		}
		for _, c := range controls {
			if c.x+c.w > width {
				t.Fatal(width, c)
			}
		}
	}
	m.Update(tabShortcut(2))
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ctrl+1")})
	if m.screen != "compose" || m.composer.Value() != "ctrl+1" {
		t.Fatal("text became shortcut")
	}
	m.Update(tabShortcut(5))
	if m.screen != "settings" {
		t.Fatal(m.screen)
	}
	m.Update(tabShortcut(2))
	if m.composer.Value() != "ctrl+1" {
		t.Fatal("draft lost")
	}
	key(m, "ctrl+n")
	m.Update(tabShortcut(1))
	if m.screen != "compose" {
		t.Fatal("navigation escaped modal")
	}
}
func TestKeyboardProtocolRestoredForChildAndExit(t *testing.T) {
	var b bytes.Buffer
	w := &keyboardOutput{Writer: &b}
	for _, seq := range []string{"\x1b[?1049h", "render", "\x1b[?1049l", "child", "\x1b[?1049h"} {
		if _, err := w.Write([]byte(seq)); err != nil {
			t.Fatal(err)
		}
	}
	w.restore()
	w.restore()
	s := b.String()
	if strings.Count(s, "\x1b[>1u") != 2 || strings.Count(s, "\x1b[<u") != 2 || !strings.Contains(s, "\x1b[<u\x1b[>4;0m\x1b[?1049lchild") {
		t.Fatal(s)
	}
}

func TestPaletteAcceptsSeparateSpaceEvents(t *testing.T) {
	m := newModel(t)
	key(m, "ctrl+p")
	for _, msg := range []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune("New")}, {Type: tea.KeySpace}, {Type: tea.KeyRunes, Runes: []rune("run")}} {
		m.Update(msg)
	}
	if m.paletteQuery != "New run" {
		t.Fatal(m.paletteQuery)
	}
	key(m, "enter")
	key(m, "task")
	if m.screen != "compose" || m.composer.Value() != "task" {
		t.Fatal("composer not focused")
	}
}
