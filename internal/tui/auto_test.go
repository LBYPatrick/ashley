package tui

import (
	"io"
	"testing"

	"github.com/LBYPatrick/ashley/internal/config"
	"github.com/charmbracelet/lipgloss"
)

func TestAutoAppearanceAndDarkFallback(t *testing.T) {
	old := lipgloss.DefaultRenderer()
	defer lipgloss.SetDefaultRenderer(old)
	// A non-terminal cannot answer the background query.
	lipgloss.SetDefaultRenderer(lipgloss.NewRenderer(io.Discard))
	m := &Model{theme: config.Theme{Mode: "auto", Preset: "blue"}}
	if m.effectiveMode() != "dark" {
		t.Fatal("unknown brightness must use dark")
	}
	for _, dark := range []bool{true, false} {
		lipgloss.SetHasDarkBackground(dark)
		want := "light"
		if dark {
			want = "dark"
		}
		m.theme.Mode = "auto"
		got := m.appearance()
		m.theme.Mode = want
		explicit := m.appearance()
		if got.bg != explicit.bg || got.fg != explicit.fg || got.accent != explicit.accent {
			t.Fatal("auto differs from resolved theme", want)
		}
		m.theme.Mode = "light"
		if m.effectiveMode() != "light" {
			t.Fatal("light override lost")
		}
		m.theme.Mode = "dark"
		if m.effectiveMode() != "dark" {
			t.Fatal("dark override lost")
		}
	}
}
