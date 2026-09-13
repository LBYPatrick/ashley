package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/LBYPatrick/ashley/internal/agents"
)

// automatedInstall is deliberately limited to installation choices, not commands.
type automatedInstall struct {
	Agents          []string `json:"agents"`
	SkillsOnly      bool     `json:"skills_only"`
	InstallSkills   bool     `json:"install_skills"`
	CommunitySkills bool     `json:"community_skills"`
}

func envEnabled(name string) bool {
	switch os.Getenv(name) {
	case "1", "true", "yes":
		return true
	}
	return false
}

func loadAutomatedInstall() (*automatedInstall, error) {
	if !envEnabled("ASHLEY_AUTOMATED") {
		return nil, nil
	}
	path := os.Getenv("ASHLEY_AUTOMATED_CONFIG")
	if path == "" {
		return nil, fmt.Errorf("ASHLEY_AUTOMATED=1 requires ASHLEY_AUTOMATED_CONFIG")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("automated installation config: %w", err)
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	var cfg automatedInstall
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("automated installation config: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("automated installation config must contain one JSON object")
	}
	if len(cfg.Agents) == 0 {
		return nil, fmt.Errorf("automated installation config requires at least one agent")
	}
	seen := map[string]bool{}
	for i, key := range cfg.Agents {
		key = strings.ToLower(strings.TrimSpace(key))
		cfg.Agents[i] = key
		if !agents.Valid(key) {
			return nil, fmt.Errorf("unknown automated installation agent: %s", key)
		}
		if seen[key] {
			return nil, fmt.Errorf("duplicate automated installation agent: %s", key)
		}
		seen[key] = true
	}
	return &cfg, nil
}
