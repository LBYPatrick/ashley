package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestLanguagePrecedence(t *testing.T) {
	for _, tc := range []struct {
		preference string
		env        map[string]string
		want       string
	}{
		{"ja", map[string]string{"LC_ALL": "ko_KR.UTF-8"}, "ja"},
		{"en", map[string]string{"LANG": "zh_CN.UTF-8"}, "en"},
		{"auto", map[string]string{"LC_ALL": "ko_KR.UTF-8", "LANG": "en_US.UTF-8"}, "ko"},
		{"auto", map[string]string{"LC_MESSAGES": "vi_VN.UTF-8", "LANG": "en_US.UTF-8"}, "vi"},
		{"auto", map[string]string{"LANG": "zh-TW.UTF-8"}, "zh"},
		{"unknown", map[string]string{"LANG": "fr_FR.UTF-8"}, "en"},
		{"auto", nil, "en"},
	} {
		if got := resolveLanguage(tc.preference, func(key string) string { return tc.env[key] }); got != tc.want {
			t.Errorf("%q with %v: got %q, want %q", tc.preference, tc.env, got, tc.want)
		}
	}
}

func TestInterfaceSettingsPersistAndKeepHelpAvailable(t *testing.T) {
	t.Setenv("LC_ALL", "ja_JP.UTF-8")
	m := newModel(t)
	m.open("settings")
	if m.language != "ja" || len(m.footerRows()) != 1 {
		t.Fatal("defaults should follow the terminal and show shortcuts")
	}
	clickSetting(t, m, "en")
	if m.language != "en" {
		t.Fatal("explicit English should override the terminal")
	}
	clickSetting(t, m, "zh")
	clickSetting(t, m, "off")
	reloaded, err := New(m.options)
	if err != nil {
		t.Fatal(err)
	}
	m.open("sessions")
	reloaded.open("sessions")
	if reloaded.language != "zh" || len(reloaded.footerRows()) != 0 || m.footerTop() != m.height {
		t.Fatal("saved interface choices were not applied")
	}
	key(reloaded, "?")
	if reloaded.screen != "help" || !strings.Contains(ansi.Strip(reloaded.View()), "重命名") {
		t.Fatalf("localized help unavailable with bar off: %s", reloaded.View())
	}
}

func TestLocalizedShortcutsFitNarrowTerminals(t *testing.T) {
	m := newModel(t)
	m.open("sessions")
	for _, language := range languages[1:] {
		m.language = language
		for _, width := range []int{20, 38, 80} {
			m.width, m.height = width, 16
			rows := m.footerRows()
			if len(rows) != 1 || ansi.StringWidth(rows[0]) > width-4 || !strings.Contains(rows[0], "?") {
				t.Fatalf("%s at width %d: %v", language, width, rows)
			}
			assertFrame(t, m.View(), width, m.height)
		}
	}
}
