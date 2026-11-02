//go:build integration && (darwin || linux)

package integration

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestAboutNavigationFromHomeAndPalette(t *testing.T) {
	s := newSandbox(t)
	s.env["NO_COLOR"] = "1"
	p := startTerminal(t, s, binary)
	p.waitFor("What would you like to work on?")
	p.send("\x10") // Ctrl+P opens the command palette.
	p.until(func() bool { return strings.Contains(ansi.Strip(p.text()), "Search for commands") })
	p.send("version")
	p.waitFor("About Ashley")
	p.send("\r")
	p.waitFor("Version: " + version)
	offset := len(p.text())
	p.send("\x1b")
	p.until(func() bool { return strings.Contains(p.text()[offset:], "What would you like to work on?") })
	offset = len(p.text())
	p.send(strings.Repeat("\x1b[B", 8) + "\r")
	p.until(func() bool { return strings.Contains(p.text()[offset:], "Version: "+version) })
	p.quit()
}
