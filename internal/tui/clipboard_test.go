package tui

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestRemoteClipboardUsesLocalTerminal(t *testing.T) {
	t.Setenv("SSH_CONNECTION", "192.0.2.1 5000 192.0.2.2 22")
	m := newModel(t)
	var out bytes.Buffer
	m.terminalOutput = &out
	text := "hello\n界"
	msg := m.copyText(text)()
	if _, ok := msg.(clipboardSent); !ok {
		t.Fatal(msg)
	}
	if out.String() != "\x1b]52;c;"+base64.StdEncoding.EncodeToString([]byte(text))+"\a" {
		t.Fatal(out.String())
	}
}
