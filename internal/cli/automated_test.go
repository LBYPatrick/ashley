package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAutomatedInstallConfig(t *testing.T) {
	t.Setenv("ASHLEY_AUTOMATED", "1")
	for _, tc := range []struct {
		data  string
		valid bool
	}{
		{`{"agents":["claude","codex"],"skills_only":true,"install_skills":true,"community_skills":true}`, true},
		{`{"agents":[" CODEX "]}`, true},
		{`{}`, false}, {`null`, false}, {`{"agents":["invalid"]}`, false},
		{`{"agents":["codex","CODEX"]}`, false}, {`{"agents":["codex"],"typo":true}`, false},
		{`{"agents":["codex"]} {}`, false}, {`{"agents":["codex"],"skills_only":"yes"}`, false},
	} {
		t.Run(tc.data, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "ashley-automated.json")
			if err := os.WriteFile(path, []byte(tc.data), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("ASHLEY_AUTOMATED_CONFIG", path)
			cfg, err := loadAutomatedInstall()
			if (err == nil) != tc.valid {
				t.Fatalf("cfg=%+v err=%v", cfg, err)
			}
		})
	}
	t.Setenv("ASHLEY_AUTOMATED_CONFIG", "")
	if _, err := loadAutomatedInstall(); err == nil {
		t.Fatal("missing path accepted")
	}
	t.Setenv("ASHLEY_AUTOMATED", "0")
	if cfg, err := loadAutomatedInstall(); err != nil || cfg != nil {
		t.Fatal("inactive config used")
	}
}

func TestCommunitySkillTargets(t *testing.T) {
	got := communitySkillCommands([]string{"claude", "codex", "grok", "opencode", "kilo"})
	want := [][]string{
		{"add", "emilkowalski/skills", "--global", "--yes", "--skill", "*", "--agent", "claude-code", "codex", "grok", "opencode", "kilo"},
		{"add", "vercel-labs/skills", "--global", "--yes", "--skill", "find-skills", "--agent", "claude-code", "codex", "grok", "opencode", "kilo"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%v", got)
	}
}

func TestAutomatedInstallNoPrompt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ASHLEY_AUTOMATED", "1")
	t.Setenv("ASHLEY_INSTALL_SKILLS", "1")
	path := filepath.Join(home, "ashley-automated.json")
	if err := os.WriteFile(path, []byte(`{"agents":["codex"],"skills_only":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ASHLEY_AUTOMATED_CONFIG", path)
	var out, stderr bytes.Buffer
	if err := Run([]string{"install"}, &out, &stderr); err != nil {
		t.Fatalf("%v %s", err, stderr.String())
	}
	if strings.Contains(out.String(), "[y/N]") || strings.Contains(out.String(), "Choice") {
		t.Fatalf("prompt: %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "skills", "a-feat", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills")); !os.IsNotExist(err) {
		t.Fatal("unselected agent installed")
	}
}
