// Package community manages skills fetched from Ashley's community bundles.
package community

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/LBYPatrick/ashley/internal/agents"
)

// Bundle identifies one selectable source of skills.
type Bundle struct {
	ID, Label, Source, Skill string
}

var bundles = []Bundle{
	{"emil", "Emil Kowalski", "emilkowalski/skills", "*"},
	{"modern-web", "Modern Web Guidance", "GoogleChrome/modern-web-guidance", "*"},
	{"find-skills", "Find Skills", "vercel-labs/skills", "find-skills"},
}

// Bundles returns the supported bundles in display order.
func Bundles() []Bundle { return slices.Clone(bundles) }

// Fetch installs a bundle into an isolated home directory.
type Fetch func(bundle Bundle, stage string) error

type record struct {
	Names   []string `json:"skills"`
	Targets []string `json:"agent_directories"`
}

func root(home string) string { return filepath.Join(home, ".ashley", "community") }

func (r record) links(directory string, visit func(path, target string) error) error {
	for _, target := range r.Targets {
		for _, name := range r.Names {
			if err := visit(filepath.Join(target, name), filepath.Join(directory, ".agents", "skills", name)); err != nil {
				return err
			}
		}
	}
	return nil
}

func readRecord(dir string) (record, bool, error) {
	var r record
	info, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		return r, false, nil
	}
	if err != nil {
		return r, false, err
	}
	if !info.IsDir() {
		return r, false, fmt.Errorf("bundle path is not a directory: %s", dir)
	}
	data, err := os.ReadFile(filepath.Join(dir, "bundle.json"))
	if err != nil {
		return r, false, err
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return r, false, err
	}
	if len(r.Names) == 0 || len(r.Targets) == 0 {
		return r, false, errors.New("empty community bundle record")
	}
	for _, name := range r.Names {
		if !filepath.IsLocal(name) || filepath.Base(name) != name || name == "." {
			return r, false, errors.New("invalid skill name in community bundle record")
		}
	}
	for _, target := range r.Targets {
		if !filepath.IsAbs(target) {
			return r, false, errors.New("invalid agent directory in community bundle record")
		}
	}
	return r, true, nil
}

// Installed returns the bundles configured through Ashley.
func Installed(home string) (map[string]bool, error) {
	checked := make(map[string]bool, len(bundles))
	for _, bundle := range bundles {
		dir := filepath.Join(root(home), bundle.ID)
		r, exists, err := readRecord(dir)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", bundle.Label, err)
		}
		if exists {
			ready := true
			err := r.links(dir, func(path, target string) error {
				for _, skill := range []string{path, target} {
					if _, err := os.Stat(filepath.Join(skill, "SKILL.md")); err != nil {
						if !os.IsNotExist(err) {
							return err
						}
						ready = false
					}
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
			if ready {
				checked[bundle.ID] = true
			}
		}
	}
	return checked, nil
}

// Apply installs selected bundles and removes deselected Ashley-managed bundles.
// Existing user files take precedence over bundled skills with matching names.
func Apply(home string, keys, desired []string, fetch Fetch, progress io.Writer) error {
	wanted := make(map[string]bool, len(desired))
	for _, id := range desired {
		if !slices.ContainsFunc(bundles, func(b Bundle) bool { return b.ID == id }) || wanted[id] {
			return fmt.Errorf("unknown or duplicate community bundle: %s", id)
		}
		wanted[id] = true
	}
	var targets []string
	for _, key := range keys {
		if !agents.Valid(key) {
			return fmt.Errorf("unknown coding agent: %s", key)
		}
		target, err := filepath.Abs(agents.SkillsDir(key, home, os.Getenv))
		if err != nil {
			return err
		}
		if !slices.Contains(targets, target) {
			targets = append(targets, target)
		}
	}
	if len(targets) == 0 && len(wanted) != 0 {
		return errors.New("select coding agents before installing community bundles")
	}
	base := root(home)
	if err := os.MkdirAll(base, 0700); err != nil {
		return err
	}
	unlock, err := lock(base)
	if err != nil {
		return err
	}
	defer unlock()

	// Remove deselected sources first so a shared skill name can move to a selected source.
	for _, bundle := range bundles {
		dir := filepath.Join(base, bundle.ID)
		r, exists, err := readRecord(dir)
		if err != nil {
			return fmt.Errorf("read %s: %w", bundle.Label, err)
		}
		if exists && !wanted[bundle.ID] {
			fmt.Fprintf(progress, "Removing %s…\n", bundle.Label)
			if err := remove(dir, r); err != nil {
				return fmt.Errorf("remove %s: %w", bundle.Label, err)
			}
		}
	}
	for _, bundle := range bundles {
		if !wanted[bundle.ID] {
			continue
		}
		dir := filepath.Join(base, bundle.ID)
		if err := ensure(base, dir, bundle, targets, fetch, progress); err != nil {
			return fmt.Errorf("install %s: %w", bundle.Label, err)
		}
	}
	return nil
}

func ensure(base, dir string, bundle Bundle, targets []string, fetch Fetch, progress io.Writer) error {
	r, exists, err := readRecord(dir)
	if err != nil {
		return err
	}
	needsFetch := !exists
	for _, name := range r.Names {
		if _, err := os.Stat(filepath.Join(dir, ".agents", "skills", name, "SKILL.md")); os.IsNotExist(err) {
			needsFetch = true
		} else if err != nil {
			return err
		}
	}
	next := record{Names: slices.Clone(r.Names), Targets: slices.Clone(r.Targets)}
	for _, target := range targets {
		if !slices.Contains(next.Targets, target) {
			next.Targets = append(next.Targets, target)
		}
	}
	if needsFetch {
		fmt.Fprintf(progress, "Installing %s…\n", bundle.Label)
		stage, err := os.MkdirTemp(base, "."+bundle.ID+"-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(stage)
		if err := fetch(bundle, stage); err != nil {
			return err
		}
		packages := filepath.Join(stage, ".agents", "skills")
		entries, err := os.ReadDir(packages)
		if err != nil {
			return fmt.Errorf("read staged skills: %w", err)
		}
		next.Names = nil
		for _, entry := range entries {
			if info, err := os.Stat(filepath.Join(packages, entry.Name(), "SKILL.md")); entry.IsDir() && err == nil && info.Mode().IsRegular() {
				next.Names = append(next.Names, entry.Name())
			}
		}
		if len(next.Names) == 0 {
			return errors.New("no skills were installed")
		}
		if err := writeRecord(stage, next); err != nil {
			return err
		}
		if exists {
			if err := remove(dir, r); err != nil {
				return err
			}
		}
		if err := os.Rename(stage, dir); err != nil {
			return err
		}
	} else if !slices.Equal(r.Targets, next.Targets) {
		if err := writeRecord(dir, next); err != nil {
			return err
		}
	}
	return next.links(dir, func(path, target string) error {
		if _, err := os.Stat(filepath.Join(target, "SKILL.md")); err != nil {
			return fmt.Errorf("read installed skill: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := os.Symlink(target, path); !errors.Is(err, os.ErrExist) {
			return err
		}
		if _, err := os.Stat(filepath.Join(path, "SKILL.md")); err != nil {
			return fmt.Errorf("another file blocks installation at %s", path)
		}
		if link, err := os.Readlink(path); err != nil || link != target {
			fmt.Fprintf(progress, "Keeping existing skill at %s.\n", path)
		}
		return nil
	})
}

func writeRecord(dir string, value record) error {
	file, err := os.CreateTemp(dir, ".bundle-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := errors.Join(json.NewEncoder(file).Encode(value), file.Close()); err != nil {
		return err
	}
	return os.Rename(file.Name(), filepath.Join(dir, "bundle.json"))
}

func remove(dir string, r record) error {
	if err := r.links(dir, func(path, target string) error {
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			return err
		}
		link, err := os.Readlink(path)
		if err != nil {
			return err
		}
		// Remove only links created by Ashley. User files and other links stay in place.
		if link == target {
			return os.Remove(path)
		}
		return nil
	}); err != nil {
		return err
	}
	return os.RemoveAll(dir)
}
