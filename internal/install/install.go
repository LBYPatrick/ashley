// Package install materializes embedded skills and links them into agent directories.
package install

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/LBYPatrick/ashley/internal/agents"
	"github.com/LBYPatrick/ashley/internal/config"
	"github.com/LBYPatrick/ashley/internal/skills"
)

// Installer manages persistent generated skills without a source checkout.
type Installer struct {
	Home    string
	Getenv  func(string) string
	Catalog skills.Catalog
}

// Result counts installed links and preserved conflicts.
type Result struct{ Installed, Skipped, Removed int }

func (i Installer) generated() string { return filepath.Join(i.Home, ".ashley", "generated") }
func (i Installer) directory(key string) string {
	getenv := i.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	return agents.SkillsDir(key, i.Home, getenv)
}
func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func atomic(path string, data []byte) error {
	return atomicMode(path, data, 0600)
}
func atomicMode(path string, data []byte, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".ashley-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, writeErr := f.Write(data)
	modeErr := f.Chmod(mode)
	closeErr := f.Close()
	if err := errors.Join(writeErr, modeErr, closeErr); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// Materialize writes prompts and complete custom packages, retaining local edits.
func (i Installer) Materialize() ([]string, error) {
	packages, err := fs.ReadDir(i.Catalog.Source, "generated")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	names, err := i.Catalog.Names()
	if err != nil && len(packages) == 0 {
		return nil, err
	}
	root := i.generated()
	if info, err := os.Lstat(root); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("generated root is a symlink: %s", root)
	}
	manifestPath := filepath.Join(root, ".manifest.json")
	manifest := map[string]string{}
	data, err := os.ReadFile(manifestPath)
	if err == nil {
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil, fmt.Errorf("invalid skill manifest: %w", err)
		}
		if manifest == nil {
			manifest = map[string]string{}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	var installed []string
	writeManaged := func(relative string, content []byte, mode fs.FileMode) error {
		destination := filepath.Join(root, relative)
		// Do not traverse links when updating nested package resources.
		for current := destination; current != root; current = filepath.Dir(current) {
			info, err := os.Lstat(current)
			if err == nil && info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("generated package path is a symlink: %s", current)
			}
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		current, err := os.ReadFile(destination)
		if err == nil && digest(current) != manifest[relative] {
			return nil
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := atomicMode(destination, content, mode); err != nil {
			return err
		}
		manifest[relative] = digest(content)
		return nil
	}
	for _, name := range names {
		result, err := i.Catalog.Assemble(name, nil)
		if err != nil {
			return nil, err
		}
		skillName := filepath.Base(filepath.Dir(result.Output))
		if skillName == "." || !filepath.IsLocal(skillName) {
			return nil, fmt.Errorf("invalid skill output: %s", result.Output)
		}
		content := []byte(result.Content)
		// Developer-provided generated documents are intentional overrides.
		if override, err := i.Catalog.Source.Open(result.Output); err == nil {
			override.Close()
			content, err = fs.ReadFile(i.Catalog.Source, result.Output)
			if err != nil {
				return nil, err
			}
		}
		relative := filepath.Join(skillName, "SKILL.md")
		if err := writeManaged(relative, content, 0600); err != nil {
			return nil, err
		}
		installed = append(installed, skillName)
	}
	for _, entry := range packages {
		if !entry.IsDir() {
			continue
		}
		packageRoot := path.Join("generated", entry.Name())
		if _, err := fs.Stat(i.Catalog.Source, path.Join(packageRoot, "SKILL.md")); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, err
		}
		err := fs.WalkDir(i.Catalog.Source, packageRoot, func(source string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("custom package file must be regular: %s", source)
			}
			content, err := fs.ReadFile(i.Catalog.Source, source)
			if err != nil {
				return err
			}
			return writeManaged(filepath.FromSlash(strings.TrimPrefix(source, "generated/")), content, info.Mode().Perm())
		})
		if err != nil {
			return nil, err
		}
		if !slices.Contains(installed, entry.Name()) {
			installed = append(installed, entry.Name())
		}
	}
	data, err = json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := atomic(manifestPath, append(data, '\n')); err != nil {
		return nil, err
	}
	// Previously imported custom packages remain available to newly selected
	// agents even when the source checkout is no longer present.
	localPackages, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, entry := range localPackages {
		if !entry.IsDir() || slices.Contains(installed, entry.Name()) {
			continue
		}
		if info, err := os.Stat(filepath.Join(root, entry.Name(), "SKILL.md")); err == nil && info.Mode().IsRegular() {
			installed = append(installed, entry.Name())
		}
	}
	slices.Sort(installed)
	return installed, nil
}
func (i Installer) owned(path string) bool {
	target, err := os.Readlink(path)
	if err != nil {
		return false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err == nil {
		target = resolved
	}
	target = filepath.Clean(target)
	rel, err := filepath.Rel(i.generated(), target)
	if err == nil && filepath.IsLocal(rel) {
		return true
	}
	return strings.Contains(filepath.ToSlash(target), "ashley/generated/")
}

// HasSkills includes both binary-managed and legacy checkout symlinks.
func (i Installer) HasSkills(key string) bool {
	entries, err := os.ReadDir(i.directory(key))
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if i.owned(filepath.Join(i.directory(key), entry.Name())) {
			return true
		}
	}
	return false
}

// Resolve selects previously installed agents or the saved default, preserving preference order.
func (i Installer) Resolve(keys []string) ([]string, error) {
	prefs := config.Store{Dir: filepath.Join(i.Home, ".ashley")}
	saved := prefs.LoadAgent()
	if len(keys) == 0 {
		for _, key := range agents.Keys() {
			if i.HasSkills(key) {
				keys = append(keys, key)
			}
		}
		if len(keys) == 0 {
			keys = []string{saved}
		}
	}
	var result []string
	for _, key := range keys {
		if !agents.Valid(key) {
			return nil, fmt.Errorf("unknown coding agent: %s", key)
		}
		key = agents.Get(key).Key
		if !slices.Contains(result, key) {
			result = append(result, key)
		}
	}
	if index := slices.Index(result, saved); index > 0 {
		result = append([]string{saved}, append(result[:index], result[index+1:]...)...)
	}
	return result, nil
}

// Install links selected agents to persistent generated skills, preserving user directories.
func (i Installer) Install(keys []string) (Result, error) {
	var result Result
	keys, err := i.Resolve(keys)
	if err != nil {
		return result, err
	}
	names, err := i.Materialize()
	if err != nil {
		return result, err
	}
	for _, key := range keys {
		directory := i.directory(key)
		if err := os.MkdirAll(directory, 0755); err != nil {
			return result, err
		}
		for _, name := range names {
			path := filepath.Join(directory, name)
			target := filepath.Join(i.generated(), name)
			info, err := os.Lstat(path)
			if err == nil {
				if info.Mode()&os.ModeSymlink == 0 || !i.owned(path) {
					result.Skipped++
					continue
				}
				existing, _ := filepath.EvalSymlinks(path)
				if existing == target {
					result.Skipped++
					continue
				}
				if err := os.Remove(path); err != nil {
					return result, err
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				return result, err
			}
			if err := os.Symlink(target, path); err != nil {
				return result, err
			}
			result.Installed++
		}
	}
	err = (config.Store{Dir: filepath.Join(i.Home, ".ashley")}).SaveAgent(keys[0])
	return result, err
}

// Uninstall removes only Ashley-owned symlinks, retaining custom files and history.
func (i Installer) Uninstall(keys []string) (Result, error) {
	result := Result{}
	if len(keys) == 0 {
		keys = agents.Keys()
	}
	for _, key := range keys {
		if !agents.Valid(key) {
			return result, fmt.Errorf("unknown coding agent: %s", key)
		}
		directory := i.directory(key)
		entries, err := os.ReadDir(directory)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return result, err
		}
		for _, entry := range entries {
			path := filepath.Join(directory, entry.Name())
			if i.owned(path) {
				if err := os.Remove(path); err != nil {
					return result, err
				}
				result.Removed++
			}
		}
	}
	return result, nil
}
