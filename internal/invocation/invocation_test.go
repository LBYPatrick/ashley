package invocation

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

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
			assertSharedRules(t, b, v)
			if installed && !strings.Contains(joined, agents.Trigger(key, "feat", "add login")) {
				t.Fatal(joined)
			}
			if installed && !strings.HasPrefix(v.Args[len(v.Args)-1], agents.Trigger(key, "feat", "add login")) {
				t.Fatal("native skill trigger must start the user prompt", v.Args)
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

func TestResumeDoesNotReplayPrompt(t *testing.T) {
	for _, agent := range agents.Keys() {
		for _, mode := range []string{"default", "auto", "dsp", "afk"} {
			b := builder(t)
			v, err := b.Build(Options{Agent: agent, Skill: "coding", Question: "do not repeat this task", ResumeID: "conversation-123", DSP: mode == "dsp", Auto: mode == "auto", AFK: mode == "afk"})
			if err != nil {
				t.Fatal(err)
			}
			flags, _ := agents.PermissionArgs(agent, mode == "dsp", mode == "auto", mode == "afk")
			resume, err := agents.ResumeArgs(agent, "conversation-123")
			if err != nil {
				t.Fatal(err)
			}
			want := append(append([]string{"/bin/" + agent}, flags...), resume...)
			if !slices.Equal(v.Args, want) || v.Permission != mode {
				t.Fatalf("%s/%s: %q", agent, mode, v.Args)
			}
		}
	}
	b := builder(t)
	if _, err := b.Build(Options{Agent: "codex", Skill: "raw", ResumeID: "../bad"}); err == nil {
		t.Fatal("accepted invalid ID")
	}
}

func assertSharedRules(t *testing.T, b Builder, v Invocation) {
	t.Helper()
	args := slices.Clone(v.Args)
	for i, arg := range args {
		if strings.HasPrefix(arg, sessions.PromptFileMarker) {
			data, err := os.ReadFile(strings.TrimPrefix(arg, sessions.PromptFileMarker))
			if err != nil {
				t.Fatal(err)
			}
			args[i] = string(data)
		}
	}
	rules, err := b.sharedRules()
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range rules {
		if count := strings.Count(strings.Join(args, "\n"), rule); count != 1 {
			t.Fatalf("want one complete shared rule, got %d: %q", count, args)
		}
	}
}

func TestSharedRulesCustomCatalogAndReadFailure(t *testing.T) {
	b := builder(t)
	path := filepath.Join(agents.SkillsDir("codex", b.Home, b.Getenv), "a-coding")
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte("custom"), 0600); err != nil {
		t.Fatal(err)
	}
	b.Catalog.Source = fstest.MapFS{"components/guidelines-wording.md": {Data: []byte("Use the custom writing rules.")}}
	v, err := b.Build(Options{Agent: "codex", Skill: "coding", Question: "Create a PDF"})
	if err != nil {
		t.Fatal(err)
	}
	assertSharedRules(t, b, v)
	if !strings.Contains(strings.Join(v.Args, "\n"), "Use the custom writing rules.") {
		t.Fatal("custom rules ignored")
	}
	// Missing custom components fall back; unreadable components must fail.
	b.Catalog.Source = fstest.MapFS{"components/guidelines-wording.md": {Mode: fs.ModeDir}}
	if _, err := b.Build(Options{Agent: "codex", Skill: "coding"}); err == nil {
		t.Fatal("unreadable output rules ignored")
	}
}

func TestAgentRulesCustomOverrideAndFallback(t *testing.T) {
	const component = "components/guidelines-agents.md"
	data, err := fs.ReadFile(ashley.Assets, component)
	if err != nil {
		t.Fatal(err)
	}
	bundled := strings.TrimSpace(string(data))
	for _, installed := range []bool{false, true} {
		for _, test := range []struct {
			name   string
			source fs.FS
			want   string
		}{
			{name: "fallback", source: fstest.MapFS{}, want: bundled},
			{name: "override", source: fstest.MapFS{component: {Data: []byte("Use custom agent rules.")}}, want: "Use custom agent rules."},
			{name: "empty override", source: fstest.MapFS{component: {Data: []byte("\n")}}},
		} {
			t.Run(fmt.Sprintf("%s/installed=%t", test.name, installed), func(t *testing.T) {
				b := builder(t)
				b.Catalog.Source = skills.Overlay{User: test.source, Base: ashley.Assets}
				if installed {
					b.Catalog.Source = test.source
					path := filepath.Join(agents.SkillsDir("codex", b.Home, b.Getenv), "a-coding")
					if err := os.MkdirAll(path, 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte("custom"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				v, err := b.Build(Options{Agent: "codex", Skill: "coding", Question: "Update the application"})
				if err != nil {
					t.Fatal(err)
				}
				assertSharedRules(t, b, v)
				joined := strings.Join(v.Args, "\n")
				if test.want != "" && strings.Count(joined, test.want) != 1 {
					t.Fatal("agent rules must appear exactly once")
				}
				if test.want != bundled && strings.Contains(joined, bundled) {
					t.Fatal("custom agent rules must replace the bundled rules")
				}
			})
		}
	}
	b := builder(t)
	b.Catalog.Source = skills.Overlay{User: fstest.MapFS{component: {Mode: fs.ModeDir}}, Base: ashley.Assets}
	if _, err := b.sharedRules(); err == nil {
		t.Fatal("unreadable agent rules ignored")
	}
}
