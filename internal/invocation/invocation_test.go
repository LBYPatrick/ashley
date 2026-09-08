package invocation

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	ashley "github.com/LBYPatrick/ashley"
	"github.com/LBYPatrick/ashley/internal/agents"
	"github.com/LBYPatrick/ashley/internal/sessions"
	"github.com/LBYPatrick/ashley/internal/skills"
)

func builder(t *testing.T) Builder {
	return Builder{Home: t.TempDir(), TempDir: t.TempDir(), Catalog: skills.Catalog{Source: ashley.Assets}, Getenv: func(string) string { return "" }, LookPath: func(name string) (string, error) { return "/bin/" + name, nil }}
}
func TestInstalledAndInlineAllAgents(t *testing.T) {
	for _, key := range agents.Keys() {
		for _, installed := range []bool{false, true} {
			b := builder(t)
			a := agents.Get(key)
			if installed {
				path := filepath.Join(agents.SkillsDir(key, b.Home, b.Getenv), "a-feat")
				os.MkdirAll(path, 0700)
				os.WriteFile(filepath.Join(path, "SKILL.md"), []byte("custom"), 0600)
			}
			v, err := b.Build(Options{Agent: key, Skill: "feat", Question: "add login", AFK: true, ExtraFlags: []string{"--model", "test"}})
			if err != nil {
				t.Fatal(key, err)
			}
			if v.Permission != "afk" || !slices.Contains(v.Args, a.DSPFlags[0]) || !slices.Contains(v.Args, "--model") {
				t.Fatal(v)
			}
			joined := strings.Join(v.Args, "\n")
			if !strings.Contains(joined, afkAddendum) {
				t.Fatal("missing autonomous instructions")
			}
			if installed && !strings.Contains(joined, agents.Trigger(key, "feat", "add login")) {
				t.Fatal(joined)
			}
			if !installed && !strings.Contains(joined, "add login") {
				t.Fatal(joined)
			}
			if a.SystemPromptFlag != "" && !slices.Contains(v.Args, a.SystemPromptFlag) {
				t.Fatal(v.Args)
			}
			if a.PromptFlag != "" && v.Args[len(v.Args)-2] != a.PromptFlag {
				t.Fatal(v.Args)
			}
		}
	}
}
func TestRawEmptyAndSpilledPrompts(t *testing.T) {
	b := builder(t)
	for _, key := range agents.Keys() {
		v, err := b.Build(Options{Agent: key, Skill: "raw"})
		if err != nil || len(v.Args) != 1 {
			t.Fatal(v, err)
		}
		v, err = b.Build(Options{Agent: key, Skill: "feat"})
		if err != nil || !strings.Contains(strings.Join(v.Args, "\n"), agents.Trigger(key, "feat", "")) {
			t.Fatal(v, err)
		}
	}
	question := strings.Repeat("x", 5000)
	v, err := b.Build(Options{Agent: "codex", Skill: "raw", Question: question, Detached: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(v.TempFiles) != 1 || !strings.HasPrefix(v.Args[len(v.Args)-1], sessions.PromptFileMarker) {
		t.Fatal(v)
	}
	path := v.TempFiles[0]
	data, err := os.ReadFile(path)
	if err != nil || string(data) != question {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("prompt is not private")
	}
	v.Cleanup()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("spilled prompt remains")
	}
	b.TempDir = filepath.Join(t.TempDir(), "missing")
	if _, err := b.Build(Options{Skill: "raw", Question: question, Detached: true}); err == nil {
		t.Fatal("spill failure ignored")
	}
}
func TestInvalidRequests(t *testing.T) {
	b := builder(t)
	for _, skill := range []string{"", "../escape", "missing-skill"} {
		if _, err := b.Build(Options{Skill: skill}); err == nil {
			t.Fatal(skill)
		}
	}
	b.LookPath = func(string) (string, error) { return "", os.ErrNotExist }
	if _, err := b.Build(Options{Skill: "raw", Agent: "grok"}); err == nil || !strings.Contains(err.Error(), "Grok Build") {
		t.Fatal(err)
	}
}

func TestLiteralPromptMarkerCannotReadOrRemoveUserFile(t *testing.T) {
	b := builder(t)
	original := filepath.Join(t.TempDir(), "important.txt")
	os.WriteFile(original, []byte("must survive"), 0600)
	question := sessions.PromptFileMarker + original
	v, err := b.Build(Options{Agent: "codex", Skill: "raw", Question: question, Detached: true})
	if err != nil {
		t.Fatal(err)
	}
	defer v.Cleanup()
	if len(v.TempFiles) != 1 || v.TempFiles[0] == original {
		t.Fatal("literal prompt interpreted as internal file reference")
	}
	data, err := os.ReadFile(v.TempFiles[0])
	if err != nil || string(data) != question {
		t.Fatal(string(data), err)
	}
	v.Cleanup()
	if data, err := os.ReadFile(original); err != nil || string(data) != "must survive" {
		t.Fatal("modified user file")
	}
}
