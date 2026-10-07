package tui

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

type clipboardSent struct{}

func remoteTerminal() bool {
	return os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != "" || os.Getenv("MOSH_IP") != ""
}
func (m *Model) copyTerminal(text string) tea.Msg {
	// Use the explicit clipboard selector and BEL terminator understood by Mosh.
	// Keep the encoded payload below common terminal clipboard limits.
	if len(text) > 64<<10 {
		return completed{fmt.Errorf("text exceeds the remote clipboard limit (64 KiB); use ash prompt to save it to a file")}
	}
	_, err := io.WriteString(m.terminalOutput, "\x1b]52;c;"+base64.StdEncoding.EncodeToString([]byte(text))+"\a")
	if err != nil {
		return completed{err}
	}
	return clipboardSent{}
}
