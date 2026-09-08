package tui

import (
	"github.com/LBYPatrick/ashley/internal/config"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func clickSetting(t *testing.T, m *Model, key string) {
	t.Helper()
	for _, c := range m.settingsControls() {
		if c.key == key {
			m.settingsRow, m.settingsColumn = c.row, c.column
			m.keepSettingVisible()
			m.Update(tea.MouseMsg{X: c.x + c.w/2, Y: c.y - m.screenScroll, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
			return
		}
	}
	t.Fatal("missing setting", key)
}
func TestSettingsResponsiveLayoutAndFocus(t *testing.T) {
	trueColor(t)
	for _, size := range [][2]int{{50, 20}, {80, 24}, {100, 32}, {190, 40}} {
		m := newModel(t)
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m.open("settings")
		controls := m.settingsControls()
		if len(controls) != 18 {
			t.Fatal("lost choices", len(controls))
		}
		if b := m.settingsBounds(); b.w > 86 || b.x < 2 {
			t.Fatal("unbounded settings column", b)
		}
		for i, c := range controls {
			if c.x < 2 || c.x+c.w > m.width-2 {
				t.Fatal("control clipped", size, c)
			}
			for _, other := range controls[i+1:] {
				if c.contains(other.x, other.y) || other.contains(c.x, c.y) {
					t.Fatal("overlapping choices", c, other)
				}
			}
		}
		seen := map[string]bool{}
		for range len(controls) {
			for _, c := range controls {
				if c.row == m.settingsRow && c.column == m.settingsColumn {
					seen[c.key] = true
					if c.y-m.screenScroll < 1 || c.y-m.screenScroll >= m.height-1 {
						t.Fatal("keyboard focus hidden", size, c)
					}
				}
			}
			rendered := m.View()
			if !strings.Contains(ansi.Strip(rendered), "›") {
				t.Fatal("focus marker missing")
			}
			key(m, "tab")
		}
		if len(seen) != len(controls) {
			t.Fatal("keyboard cannot reach all settings", seen)
		}
		key(m, "shift+tab")
		if m.settingsRow != 4 {
			t.Fatal("reverse tab did not reach Done")
		}
		if size[0] == 100 {
			m.focusSettings()
			m.screenScroll = 0
			exportRegressionView(t, "settings-redesigned", m.View())
			text := ansi.Strip(m.View())
			for _, label := range []string{"Settings", "Coding agent", "Appearance", "Accent color", "Purple", "Grape", "OpenAI Codex", "Done"} {
				if !strings.Contains(text, label) {
					t.Fatal("label lost", label)
				}
			}
		}
	}
}
func TestSettingsFocusAndSelectionAreIndependent(t *testing.T) {
	m := newModel(t)
	m.open("settings")
	key(m, "down")
	if m.agent != "claude" || m.settingsColumn != 1 {
		t.Fatal("navigation changed selection")
	}
	text := ansi.Strip(m.View())
	if !strings.Contains(text, "› OpenAI Codex") || !strings.Contains(text, "✓") {
		t.Fatal("focus and selection need separate indicators", text)
	}
	key(m, "enter")
	if m.prefs().LoadAgent() != "codex" {
		t.Fatal("selection was not saved immediately")
	}
	clickSetting(t, m, "forest")
	m.Update(tea.WindowSizeMsg{Width: 50, Height: 20})
	if !strings.Contains(ansi.Strip(m.View()), "Forest") {
		t.Fatal("resize lost focused preset")
	}
	clickSetting(t, m, "done")
	if m.screen != "hub" {
		t.Fatal("Done failed")
	}
}

func TestSettingsAllThemesAndFirstRun(t *testing.T) {
	trueColor(t)
	m := newModel(t)
	m.open("settings")
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 32})
	for _, mode := range []string{"dark", "light"} {
		for _, preset := range config.Presets() {
			m.theme = config.Theme{Mode: mode, Preset: preset.Key}
			text := ansi.Strip(m.View())
			if strings.Count(text, "✓") != 3 {
				t.Fatal("agent, mode and palette must show independent selections", mode, preset.Key)
			}
			if mode == "light" && preset.Key == "blue" {
				exportRegressionView(t, "settings-light", m.View())
			}
		}
	}
	os.Remove(filepath.Join(m.options.Home, ".ashley", "theme.json"))
	first, err := New(Options{Home: m.options.Home, Catalog: m.options.Catalog, Screen: "vibe"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ansi.Strip(first.View()), "Welcome to Ashley") {
		t.Fatal("onboarding title missing")
	}
	clickSetting(t, first, "kilo")
	clickSetting(t, first, "light")
	clickSetting(t, first, "ocean")
	clickSetting(t, first, "done")
	if first.screen != "vibe" || first.prefs().LoadAgent() != "kilo" || first.prefs().LoadTheme().Preset != "ocean" {
		t.Fatal("onboarding lost choices or destination")
	}
}
