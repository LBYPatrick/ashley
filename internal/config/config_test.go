package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func write(t *testing.T, s Store, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(s.Dir, name), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
func TestConfigAndHookOverrides(t *testing.T) {
	s := Store{t.TempDir()}
	write(t, s, "config.yaml", `defaults:
  permission_mode: auto
hooks:
  global:
    before_run: echo before
    after_run: [echo after]
    on_error: echo error
  skills:
    feat:
      before_run: [make test]
      after_run: []
    ignored: nonsense
pipelines:
  ship: [feat, commit]
  invalid: feat
extension: preserved
`)
	c := s.Load()
	if c.PermissionMode != "auto" || c.Raw["extension"] != "preserved" {
		t.Fatal(c)
	}
	if !reflect.DeepEqual(c.ResolvePipeline("ship"), []string{"feat", "commit"}) || !reflect.DeepEqual(c.ResolvePipeline(" feat + + commit "), []string{"feat", "commit"}) {
		t.Fatal(c.Pipelines)
	}
	h := c.HooksFor("feat")
	if !reflect.DeepEqual(h.BeforeRun, []string{"make test"}) || !reflect.DeepEqual(h.AfterRun, []string{"echo after"}) || !reflect.DeepEqual(h.OnError, []string{"echo error"}) {
		t.Fatal(h)
	}
	if !reflect.DeepEqual(c.HooksFor("missing"), c.GlobalHooks) {
		t.Fatal("global hooks lost")
	}
	if _, ok := c.Pipelines["invalid"]; ok {
		t.Fatal("accepted non-list pipeline")
	}
	before, _ := os.ReadFile(filepath.Join(s.Dir, "config.yaml"))
	if _, err := s.Init(); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(s.Dir, "config.yaml"))
	if string(before) != string(after) {
		t.Fatal("overwrote custom YAML")
	}
}
func TestMalformedAndMissing(t *testing.T) {
	s := Store{t.TempDir()}
	if s.Load().PermissionMode != "default" {
		t.Fatal("missing defaults")
	}
	for _, data := range []string{"[bad", "[]", "null", "", "defaults: 5\nhooks: false\npipelines: text"} {
		write(t, s, "config.yaml", data)
		if s.Load().PermissionMode != "default" {
			t.Fatal(data)
		}
	}
	for _, data := range []string{"{bad", "[]", "null", `{"agent":42,"mode":false,"preset":[]}`, `{"agent":"unknown","mode":"neon","preset":"unknown"}`} {
		write(t, s, "prefs.json", data)
		write(t, s, "theme.json", data)
		if s.LoadAgent() != "claude" || s.LoadTheme() != (Theme{"dark", "blue"}) {
			t.Fatal(data)
		}
	}
}
func TestPreferencesPreserveUserData(t *testing.T) {
	s := Store{t.TempDir()}
	if s.ThemeConfigured() {
		t.Fatal("unexpected saved theme")
	}
	if _, err := s.Init(); err != nil {
		t.Fatal(err)
	}
	yamlBefore, _ := os.ReadFile(filepath.Join(s.Dir, "config.yaml"))
	if !strings.Contains(string(yamlBefore), "# Hooks") {
		t.Fatal("missing template")
	}
	write(t, s, "prefs.json", `{"extension":{"enabled":true},"agent":"codex"}`)
	if err := s.SaveAgent(" KILO "); err != nil {
		t.Fatal(err)
	}
	if s.LoadAgent() != "kilo" || s.readJSON("prefs.json")["extension"] == nil {
		t.Fatal("preference data lost")
	}
	if err := s.SaveAgent("unknown"); err == nil {
		t.Fatal("accepted unknown agent")
	}
	for _, p := range Presets() {
		for _, mode := range []string{"light", "dark"} {
			want := Theme{mode, p.Key}
			if err := s.SaveTheme(want); err != nil {
				t.Fatal(err)
			}
			if s.LoadTheme() != want || !s.ThemeConfigured() {
				t.Fatal(want)
			}
		}
	}
	if s.SaveTheme(Theme{"invalid", "blue"}) == nil || s.SaveTheme(Theme{"dark", "invalid"}) == nil {
		t.Fatal("invalid theme accepted")
	}
	yamlAfter, _ := os.ReadFile(filepath.Join(s.Dir, "config.yaml"))
	if string(yamlBefore) != string(yamlAfter) {
		t.Fatal("preferences rewrote YAML")
	}
}
func TestConfigFailures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	s := Store{path}
	if _, err := s.Init(); err == nil {
		t.Fatal("accepted invalid directory")
	}
	if err := s.SaveAgent("codex"); err == nil {
		t.Fatal("ignored write failure")
	}
	s = Store{t.TempDir()}
	if err := os.Mkdir(filepath.Join(s.Dir, "config.yaml"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Init(); err == nil {
		t.Fatal("accepted directory config")
	}
}
func TestScalarCompatibility(t *testing.T) {
	s := Store{t.TempDir()}
	write(t, s, "config.yaml", "hooks:\n  global:\n    before_run: [true, false, null, 42]\npipelines:\n  empty: []")
	if !reflect.DeepEqual(s.Load().GlobalHooks.BeforeRun, []string{"True", "False", "None", "42"}) {
		t.Fatal(s.Load())
	}
	if len(s.Load().ResolvePipeline("empty")) != 0 {
		t.Fatal("empty pipeline expanded to a skill")
	}
}
