// Release identity validation and version synchronization for maintainers.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-(alpha|beta|rc)\.(0|[1-9][0-9]*))?$`)
var badgePattern = regexp.MustCompile(`img\.shields\.io/badge/version-[^" ]+-blue`)

func validateVersion(version string) error {
	if !versionPattern.MatchString(version) {
		return fmt.Errorf("use X.Y.Z or X.Y.Z-alpha.N / beta.N / rc.N")
	}
	return nil
}
func badge(version string) string {
	return "img.shields.io/badge/version-" + strings.ReplaceAll(version, "-", "--") + "-blue"
}
func prepare(root, version string) error {
	if err := validateVersion(version); err != nil {
		return err
	}
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		return err
	}
	if len(badgePattern.FindAll(readme, -1)) != 1 {
		return fmt.Errorf("expected exactly one README version badge")
	}
	readme = badgePattern.ReplaceAllLiteral(readme, []byte(badge(version)))
	if err := os.WriteFile(filepath.Join(root, "README.md"), readme, 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "VERSION"), []byte(version+"\n"), 0644)
}
func check(root, tag string) (string, error) {
	if !strings.HasPrefix(tag, "v") {
		return "", fmt.Errorf("release tags must start with v")
	}
	version := strings.TrimPrefix(tag, "v")
	if err := validateVersion(version); err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(data)) != version {
		return "", fmt.Errorf("tag and VERSION disagree")
	}
	data, err = os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		return "", err
	}
	if matches := badgePattern.FindAllString(string(data), -1); len(matches) != 1 || matches[0] != badge(version) {
		return "", fmt.Errorf("tag and README version badge disagree")
	}
	data, err = os.ReadFile(filepath.Join(root, "docs/migration/status.json"))
	if err != nil {
		return "", err
	}
	var status map[string]any
	if err := json.Unmarshal(data, &status); err != nil || len(status) == 0 {
		return "", fmt.Errorf("migration status must be a nonempty map of feature booleans")
	}
	for key, value := range status {
		complete, ok := value.(bool)
		if !ok {
			return "", fmt.Errorf("migration status must contain feature booleans")
		}
		if !complete && !strings.Contains(version, "-") {
			return "", fmt.Errorf("stable release requires full feature parity: %s is pending", key)
		}
	}
	return version, nil
}
func run(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: go run ./scripts/release <prepare|check> <version-or-tag>")
	}
	switch args[0] {
	case "prepare":
		return prepare(".", args[1])
	case "check":
		version, err := check(".", args[1])
		if err == nil {
			fmt.Println(version)
		}
		return err
	default:
		return fmt.Errorf("unknown release command: %s", args[0])
	}
}
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Release validation failed:", err)
		os.Exit(1)
	}
}
