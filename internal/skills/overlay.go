package skills

import (
	"errors"
	"io/fs"
	"sort"
	"strings"
)

// Overlay reads user-authored definitions and resources before embedded defaults.
// Generated output is excluded so release upgrades can regenerate built-in skills.
type Overlay struct{ User, Base fs.FS }

func userPath(name string) bool {
	return name == "skills" || name == "components" || name == "res" || strings.HasPrefix(name, "skills/") || strings.HasPrefix(name, "components/") || strings.HasPrefix(name, "res/")
}

// Open resolves custom source files before bundled files.
func (o Overlay) Open(name string) (fs.File, error) {
	if userPath(name) {
		file, err := o.User.Open(name)
		if err == nil {
			return file, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	return o.Base.Open(name)
}

// ReadDir merges user and built-in directory entries by name.
func (o Overlay) ReadDir(name string) ([]fs.DirEntry, error) {
	base, baseErr := fs.ReadDir(o.Base, name)
	if !userPath(name) {
		return base, baseErr
	}
	user, userErr := fs.ReadDir(o.User, name)
	if userErr != nil && !errors.Is(userErr, fs.ErrNotExist) {
		return nil, userErr
	}
	if baseErr != nil && !errors.Is(baseErr, fs.ErrNotExist) {
		return nil, baseErr
	}
	if baseErr != nil && userErr != nil {
		return nil, baseErr
	}
	entries := map[string]fs.DirEntry{}
	for _, entry := range base {
		entries[entry.Name()] = entry
	}
	for _, entry := range user {
		entries[entry.Name()] = entry
	}
	result := make([]fs.DirEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name() < result[j].Name() })
	return result, nil
}
