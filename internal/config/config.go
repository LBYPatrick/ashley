// Package config preserves Ashley's YAML configuration and JSON preferences.
package config

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LBYPatrick/ashley/internal/agents"
	"go.yaml.in/yaml/v3"
)

//go:embed default.yaml
var defaultConfig []byte

// Hooks defines commands at each invocation lifecycle point.
type Hooks struct{ BeforeRun, AfterRun, OnError []string }

// Config contains user settings and unrecognized data for future extensions.
type Config struct {
	PermissionMode string
	GlobalHooks    Hooks
	SkillHooks     map[string]Hooks
	Pipelines      map[string][]string
	Raw            map[string]any
}

// Store locates configuration independently of the source checkout.
type Store struct{ Dir string }

// User returns the current user's configuration store.
func User() (Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Store{}, err
	}
	return Store{Dir: filepath.Join(home, ".ashley")}, nil
}
func defaults() Config {
	return Config{PermissionMode: "default", SkillHooks: map[string]Hooks{}, Pipelines: map[string][]string{}, Raw: map[string]any{}}
}
func mapping(value any) map[string]any { m, _ := value.(map[string]any); return m }
func pythonString(value any) string {
	switch v := value.(type) {
	case nil:
		return "None"
	case bool:
		if v {
			return "True"
		}
		return "False"
	default:
		return fmt.Sprint(v)
	}
}
func commands(value any) []string {
	switch v := value.(type) {
	case string:
		return []string{v}
	case []any:
		result := make([]string, len(v))
		for i, x := range v {
			result[i] = pythonString(x)
		}
		return result
	default:
		return nil
	}
}
func parseHooks(value any) Hooks {
	m := mapping(value)
	return Hooks{commands(m["before_run"]), commands(m["after_run"]), commands(m["on_error"])}
}

// Load returns defaults when configuration is absent or malformed.
func (s Store) Load() Config {
	c := defaults()
	data, err := os.ReadFile(filepath.Join(s.Dir, "config.yaml"))
	if err != nil {
		return c
	}
	var raw map[string]any
	if yaml.Unmarshal(data, &raw) != nil || raw == nil {
		return c
	}
	c.Raw = raw
	if mode, ok := mapping(raw["defaults"])["permission_mode"].(string); ok {
		c.PermissionMode = mode
	}
	hooks := mapping(raw["hooks"])
	c.GlobalHooks = parseHooks(hooks["global"])
	for skill, value := range mapping(hooks["skills"]) {
		if mapping(value) != nil {
			c.SkillHooks[skill] = parseHooks(value)
		}
	}
	for name, steps := range mapping(raw["pipelines"]) {
		if _, ok := steps.([]any); ok {
			c.Pipelines[name] = commands(steps)
		}
	}
	return c
}

// HooksFor resolves per-point overrides, falling back on global hooks for empty lists.
func (c Config) HooksFor(skill string) Hooks {
	h := c.SkillHooks[skill]
	if len(h.BeforeRun) == 0 {
		h.BeforeRun = c.GlobalHooks.BeforeRun
	}
	if len(h.AfterRun) == 0 {
		h.AfterRun = c.GlobalHooks.AfterRun
	}
	if len(h.OnError) == 0 {
		h.OnError = c.GlobalHooks.OnError
	}
	return h
}

// Init creates a commented config without overwriting user-authored settings.
func (s Store) Init() (string, error) {
	path := filepath.Join(s.Dir, "config.yaml")
	if err := os.MkdirAll(s.Dir, 0700); err != nil {
		return "", err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		info, e := os.Stat(path)
		if e != nil {
			return "", e
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("configuration is not a file: %s", path)
		}
		return path, nil
	}
	if err != nil {
		return "", err
	}
	_, writeErr := f.Write(defaultConfig)
	closeErr := f.Close()
	return path, errors.Join(writeErr, closeErr)
}
func (s Store) readJSON(name string) map[string]any {
	data, err := os.ReadFile(filepath.Join(s.Dir, name))
	if err != nil {
		return map[string]any{}
	}
	var result map[string]any
	if json.Unmarshal(data, &result) != nil || result == nil {
		return map[string]any{}
	}
	return result
}
func (s Store) writeJSON(name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.Dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(s.Dir, ".prefs-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, writeErr := f.Write(append(data, '\n'))
	closeErr := f.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(s.Dir, name))
}

// LoadAgent reads legacy preferences and validates the selected backend.
func (s Store) LoadAgent() string {
	key, _ := s.readJSON("prefs.json")["agent"].(string)
	return agents.Get(key).Key
}

// SaveAgent preserves unrelated preference keys and the YAML configuration.
func (s Store) SaveAgent(key string) error {
	if !agents.Valid(key) {
		return fmt.Errorf("unknown coding agent: %s", key)
	}
	prefs := s.readJSON("prefs.json")
	prefs["agent"] = agents.Get(key).Key
	return s.writeJSON("prefs.json", prefs)
}

// Theme stores the light/dark mode and named color palette.
type Theme struct {
	Mode   string `json:"mode"`
	Preset string `json:"preset"`
}

func validPreset(key string) bool {
	for _, p := range Presets() {
		if p.Key == key {
			return true
		}
	}
	return false
}

// LoadTheme reads saved appearance settings with validated fallbacks.
func (s Store) LoadTheme() Theme {
	data := s.readJSON("theme.json")
	mode, _ := data["mode"].(string)
	preset, _ := data["preset"].(string)
	if mode != "dark" && mode != "light" {
		mode = "dark"
	}
	if !validPreset(preset) {
		preset = "blue"
	}
	return Theme{mode, preset}
}

// ThemeConfigured reports whether appearance setup has been saved.
func (s Store) ThemeConfigured() bool {
	info, err := os.Stat(filepath.Join(s.Dir, "theme.json"))
	return err == nil && info.Mode().IsRegular()
}

// SaveTheme persists appearance without editing hooks or pipelines.
func (s Store) SaveTheme(theme Theme) error {
	if theme.Mode != "dark" && theme.Mode != "light" {
		return fmt.Errorf("unknown theme mode: %s", theme.Mode)
	}
	if !validPreset(theme.Preset) {
		return fmt.Errorf("unknown theme preset: %s", theme.Preset)
	}
	return s.writeJSON("theme.json", theme)
}

// ResolvePipeline expands a named pipeline or a plus-separated chain.
func (c Config) ResolvePipeline(name string) []string {
	if !strings.Contains(name, "+") {
		if steps, ok := c.Pipelines[name]; ok {
			return append([]string{}, steps...)
		}
	}
	var steps []string
	for _, step := range strings.Split(name, "+") {
		if step = strings.TrimSpace(step); step != "" {
			steps = append(steps, step)
		}
	}
	return steps
}
