// Package invocation constructs coding-agent commands from Ashley skills.
package invocation

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LBYPatrick/ashley/internal/agents"
	"github.com/LBYPatrick/ashley/internal/sessions"
	"github.com/LBYPatrick/ashley/internal/skills"
)

//go:embed afk.txt
var afkAddendum string

// Builder resolves binaries and skills using injectable filesystem locations.
type Builder struct {
	Catalog       skills.Catalog
	Home, TempDir string
	Getenv        func(string) string
	LookPath      func(string) (string, error)
}

// Options selects a backend, skill, permissions and optional extra CLI arguments.
type Options struct {
	Agent, Skill, Question   string
	DSP, Auto, AFK, Detached bool
	// Normal explicitly overrides the configured default permission mode.
	Normal     bool
	ExtraFlags []string
	Project    map[string]any
}

// Invocation contains argv and any spilled prompt files owned by the caller.
type Invocation struct {
	Args       []string
	Permission string
	TempFiles  []string
}

// Cleanup removes spilled prompts after launch failure or normal completion.
func (v Invocation) Cleanup() {
	for _, path := range v.TempFiles {
		os.Remove(path)
	}
}

// Build constructs a native skill trigger or inlines the assembled instructions.
func (b Builder) Build(o Options) (v Invocation, err error) {
	lookup := b.LookPath
	if lookup == nil {
		lookup = agents.FindBinary
	}
	getenv := b.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	a := agents.Get(o.Agent)
	binary, err := lookup(a.Binary)
	if err != nil {
		return v, fmt.Errorf("%s not found; install it first: %s", a.Label, a.DocsURL)
	}
	home := b.Home
	if home == "" {
		home, err = os.UserHomeDir()
		if err != nil {
			return v, err
		}
	}
	o.Skill = strings.TrimPrefix(o.Skill, "a-")
	if o.Skill == "" || !filepath.IsLocal(o.Skill) || strings.ContainsAny(o.Skill, "/\\") {
		return v, fmt.Errorf("invalid skill name: %s", o.Skill)
	}
	installed := false
	if o.Skill != "raw" {
		for _, name := range []string{"a-" + o.Skill, o.Skill} {
			info, e := os.Stat(filepath.Join(agents.SkillsDir(a.Key, home, getenv), name, "SKILL.md"))
			if e == nil && info.Mode().IsRegular() {
				installed = true
				break
			}
		}
	}
	flags, mode := agents.PermissionArgs(a.Key, o.DSP, o.Auto, o.AFK)
	v.Args = append([]string{binary}, flags...)
	v.Permission = mode
	v.Args = append(v.Args, o.ExtraFlags...)
	instructions := []string{}
	if o.AFK {
		instructions = append(instructions, afkAddendum)
	}
	user := o.Question
	if installed {
		user = agents.Trigger(a.Key, o.Skill, o.Question)
	} else if o.Skill != "raw" {
		document, e := b.Catalog.Document(o.Skill)
		if e != nil {
			return v, e
		}
		prompt, e := skills.Prompt(document, "", o.Project)
		if e != nil {
			return v, e
		}
		instructions = append([]string{prompt}, instructions...)
		if user == "" {
			user = agents.Trigger(a.Key, o.Skill, "")
		}
	}
	system := strings.Join(instructions, "\n\n")
	defer func() {
		if err != nil {
			v.Cleanup()
		}
	}()
	textArg := func(text string) (string, error) {
		if !o.Detached || (len([]rune(text)) <= 4000 && !strings.HasPrefix(text, sessions.PromptFileMarker)) {
			return text, nil
		}
		f, e := os.CreateTemp(b.TempDir, "ashley-prompt-*.md")
		if e != nil {
			return "", e
		}
		v.TempFiles = append(v.TempFiles, f.Name())
		if _, e = f.WriteString(text); e != nil {
			f.Close()
			return "", e
		}
		if e = f.Close(); e != nil {
			return "", e
		}
		return sessions.PromptFileMarker + f.Name(), nil
	}
	if system != "" && a.SystemPromptFlag != "" {
		var text string
		text, err = textArg(system)
		if err != nil {
			return v, err
		}
		v.Args = append(v.Args, a.SystemPromptFlag, text)
	} else if system != "" {
		if user != "" {
			user = system + "\n\n" + user
		} else {
			user = system
		}
	}
	if user != "" {
		if a.PromptFlag != "" {
			v.Args = append(v.Args, a.PromptFlag)
		}
		var text string
		text, err = textArg(user)
		if err != nil {
			return v, err
		}
		v.Args = append(v.Args, text)
	}
	return v, nil
}
