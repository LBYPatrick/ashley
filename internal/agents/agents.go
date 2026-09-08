package agents

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// Get resolves an agent, falling back to Claude for missing or unknown keys.
func Get(key string) Agent {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, a := range registry {
		if a.Key == key {
			return clone(a)
		}
	}
	return clone(registry[0])
}
func clone(a Agent) Agent {
	a.SelfUpdateArgs = slices.Clone(a.SelfUpdateArgs)
	a.DSPFlags = slices.Clone(a.DSPFlags)
	a.AutoFlags = slices.Clone(a.AutoFlags)
	return a
}

// Keys returns supported agents in display order.
func Keys() []string {
	keys := make([]string, len(registry))
	for i, a := range registry {
		keys[i] = a.Key
	}
	return keys
}

// Valid reports whether the key names a supported agent.
func Valid(key string) bool { return slices.Contains(Keys(), strings.ToLower(strings.TrimSpace(key))) }

// SkillsDir resolves an agent's directory using its environment overrides.
func SkillsDir(key, home string, getenv func(string) string) string {
	a := Get(key)
	base := filepath.Join(home, a.HomeDir)
	override := ""
	if a.HomeEnv != "" {
		override = getenv(a.HomeEnv)
	}
	if override != "" {
		base = override
		if base == "~" {
			base = home
		} else if strings.HasPrefix(base, "~/") {
			base = filepath.Join(home, base[2:])
		}
	} else if a.Key == "opencode" && getenv("XDG_CONFIG_HOME") != "" {
		base = filepath.Join(getenv("XDG_CONFIG_HOME"), "opencode")
	}
	return filepath.Join(base, "skills")
}

// PermissionArgs maps run modes to backend flags; AFK and DSP take precedence.
func PermissionArgs(key string, dsp, auto, afk bool) ([]string, string) {
	a := Get(key)
	if afk {
		return a.DSPFlags, "afk"
	}
	if dsp {
		return a.DSPFlags, "dsp"
	}
	if auto {
		return a.AutoFlags, "auto"
	}
	return []string{}, "default"
}

// Trigger names an installed skill and appends the initial question.
func Trigger(key, skill, question string) string {
	text := Get(key).SkillTrigger + "a-" + strings.TrimPrefix(skill, "a-")
	if question != "" {
		text += " " + question
	}
	return text
}

// Select validates mutually exclusive agent options.
func Select(keys ...string) (string, error) {
	selected := ""
	for _, key := range keys {
		if key == "" {
			continue
		}
		if !Valid(key) {
			return "", fmt.Errorf("unknown coding agent: %s", key)
		}
		if selected != "" {
			return "", fmt.Errorf("choose only one coding agent per run")
		}
		selected = Get(key).Key
	}
	return selected, nil
}
