package tui

import (
	"io"
	"reflect"
	"strconv"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

type tabShortcut int

// Bubble Tea v1 delivers unsupported CSI sequences as a private byte-slice
// message. Keep that compatibility boundary here, without intercepting text
// or bracketed paste. The PTY tests exercise the actual library decoder.
func extendedKey(msg tea.Msg) tea.Msg {
	v := reflect.ValueOf(msg)
	if !v.IsValid() || v.Type().PkgPath() != "github.com/charmbracelet/bubbletea" || v.Type().Name() != "unknownCSISequenceMsg" {
		return msg
	}
	return decodeExtendedKey(string(v.Bytes()), msg)
}

func decodeExtendedKey(sequence string, fallback tea.Msg) tea.Msg {
	if !strings.HasPrefix(sequence, "\x1b[") {
		return fallback
	}
	body := sequence[2:]
	var code, modifier int
	var err error
	if strings.HasSuffix(body, "u") {
		parts := strings.Split(strings.TrimSuffix(body, "u"), ";")
		if len(parts) < 1 || len(parts) > 2 {
			return fallback
		}
		code, err = strconv.Atoi(strings.Split(parts[0], ":")[0])
		if err != nil {
			return fallback
		}
		modifier = 1
		if len(parts) == 2 {
			mods := strings.Split(parts[1], ":")
			modifier, err = strconv.Atoi(mods[0])
			if err != nil {
				return fallback
			}
			// Release events do not trigger navigation or insert text.
			if len(mods) > 1 && mods[1] == "3" {
				return nil
			}
		}
	} else if strings.HasPrefix(body, "27;") && strings.HasSuffix(body, "~") {
		parts := strings.Split(strings.TrimSuffix(body, "~"), ";")
		if len(parts) != 3 {
			return fallback
		}
		modifier, err = strconv.Atoi(parts[1])
		if err != nil {
			return fallback
		}
		code, err = strconv.Atoi(parts[2])
		if err != nil {
			return fallback
		}
	} else {
		return fallback
	}
	if modifier == 5 && code >= '1' && code <= '5' {
		return tabShortcut(code - '0')
	}
	if modifier < 1 || modifier > 8 {
		return fallback
	}
	alt := (modifier-1)&2 != 0
	ctrl := (modifier-1)&4 != 0
	if code == 9 && modifier == 2 {
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	}
	if ctrl && code == 32 {
		code = 0
	}
	if ctrl && code == 127 {
		code = 8
	}
	if ctrl && code >= 64 && code <= 127 {
		code &= 31
	}
	if ctrl && code >= 32 {
		return fallback
	}
	switch code {
	case 9, 13, 27, 127:
		return tea.KeyMsg{Type: tea.KeyType(code), Alt: alt}
	}
	if code >= 0 && code < 32 {
		return tea.KeyMsg{Type: tea.KeyType(code), Alt: alt}
	}
	if code >= 32 && code <= 0x10FFFF && !(code >= 0xD800 && code <= 0xF8FF) {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune(code)}, Alt: alt}
	}
	return fallback
}

// Keyboard enhancements are scoped to the TUI's alternate screen. ExecProcess
// leaves/re-enters that screen, so child agents receive their normal keyboard.
type keyboardOutput struct {
	io.Writer
	mu     sync.Mutex
	active bool
}

func (w *keyboardOutput) Fd() uintptr {
	if file, ok := w.Writer.(interface{ Fd() uintptr }); ok {
		return file.Fd()
	}
	return ^uintptr(0)
}

// Preserve term.File detection so Bubble Tea still queries dimensions and
// handles SIGWINCH. The wrapper does not own or close the caller's terminal.
func (w *keyboardOutput) Read(p []byte) (int, error) {
	if reader, ok := w.Writer.(io.Reader); ok {
		return reader.Read(p)
	}
	return 0, io.EOF
}
func (w *keyboardOutput) Close() error { return nil }

func (w *keyboardOutput) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	s := string(p)
	if strings.Contains(s, "\x1b[?1049l") && w.active {
		s = strings.ReplaceAll(s, "\x1b[?1049l", "\x1b[<u\x1b[>4;0m\x1b[?1049l")
		w.active = false
	}
	if strings.Contains(s, "\x1b[?1049h") && !w.active {
		s = strings.ReplaceAll(s, "\x1b[?1049h", "\x1b[?1049h\x1b[>4;2m\x1b[>1u")
		w.active = true
	}
	_, err := io.WriteString(w.Writer, s)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}
func (w *keyboardOutput) restore() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.active {
		io.WriteString(w.Writer, "\x1b[<u\x1b[>4;0m")
		w.active = false
	}
}
