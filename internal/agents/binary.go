package agents

import (
	"os"
	"os/exec"
	"path/filepath"
)

// FindBinary also checks vendor-native locations after a fresh installation,
// before the user's shell has picked up changes to PATH.
func FindBinary(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err == nil {
		return path, nil
	}
	if !Valid(name) {
		return "", err
	}
	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		return "", err
	}
	directories := []string{filepath.Join(home, ".local", "bin")}
	switch name {
	case "grok":
		dir := os.Getenv("GROK_BIN_DIR")
		if dir == "" {
			dir = filepath.Join(home, ".grok", "bin")
		}
		directories = append(directories, dir)
	case "opencode":
		directories = append(directories, filepath.Join(home, ".opencode", "bin"))
	}
	for _, directory := range directories {
		candidate := filepath.Join(directory, name)
		if info, e := os.Stat(candidate); e == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
			return candidate, nil
		}
	}
	return "", err
}
