// Package project detects the project context consumed by Ashley templates.
package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Detect returns the same technology flags and metadata as the Python detector.
func Detect(root string) map[string]any {
	result := make(map[string]any)
	for _, key := range strings.Fields("python typescript javascript go rust java dart c cpp shell react nextjs vue svelte fastapi django flask express flutter docker makefile git ci_github ci_gitlab npm pnpm yarn uv pip cargo has_tests has_readme") {
		result[key] = false
	}
	result["name"] = filepath.Base(filepath.Clean(root))
	file := func(name string) bool {
		info, err := os.Stat(filepath.Join(root, name))
		return err == nil && !info.IsDir()
	}
	dir := func(name string) bool {
		info, err := os.Stat(filepath.Join(root, name))
		return err == nil && info.IsDir()
	}
	if file("pyproject.toml") {
		result["python"] = true
		content, _ := os.ReadFile(filepath.Join(root, "pyproject.toml"))
		for _, dep := range []string{"fastapi", "django", "flask"} {
			if strings.Contains(strings.ToLower(string(content)), dep) {
				result[dep] = true
			}
		}
	}
	if file("setup.py") || file("setup.cfg") {
		result["python"] = true
	}
	if file("requirements.txt") {
		result["python"], result["pip"] = true, true
	}
	if file("uv.lock") {
		result["python"], result["uv"] = true, true
	}
	if file("package.json") {
		result["javascript"] = true
		data, _ := os.ReadFile(filepath.Join(root, "package.json"))
		var pkg struct {
			Name            *string        `json:"name"`
			Dependencies    map[string]any `json:"dependencies"`
			DevDependencies map[string]any `json:"devDependencies"`
		}
		if json.Unmarshal(data, &pkg) == nil {
			for _, deps := range []map[string]any{pkg.Dependencies, pkg.DevDependencies} {
				for dep := range deps {
					switch dep {
					case "typescript", "react", "vue", "svelte", "express":
						result[dep] = true
					case "react-dom":
						result["react"] = true
					case "next":
						result["nextjs"], result["react"] = true, true
					}
				}
			}
			if pkg.Name != nil {
				result["name"] = *pkg.Name
			}
		}
	}
	if file("tsconfig.json") {
		result["typescript"] = true
	}
	for _, pm := range []struct{ name, key string }{{"pnpm-lock.yaml", "pnpm"}, {"yarn.lock", "yarn"}, {"package-lock.json", "npm"}} {
		if file(pm.name) {
			result[pm.key] = true
			break
		}
	}
	for name, key := range map[string]string{"go.mod": "go", "CMakeLists.txt": "cpp", "Makefile": "makefile", ".gitlab-ci.yml": "ci_gitlab"} {
		if file(name) {
			result[key] = true
		}
	}
	if file("Cargo.toml") {
		result["rust"], result["cargo"] = true, true
	}
	if file("pom.xml") || file("build.gradle") {
		result["java"] = true
	}
	if file("pubspec.yaml") {
		result["dart"] = true
		content, _ := os.ReadFile(filepath.Join(root, "pubspec.yaml"))
		if strings.Contains(string(content), "flutter") {
			result["flutter"] = true
		}
	}
	result["docker"] = file("Dockerfile") || file("docker-compose.yml")
	_, err := os.Stat(filepath.Join(root, ".git"))
	result["git"] = err == nil
	result["ci_github"] = dir(".github/workflows")
	for _, pattern := range []string{"*.sh", "scripts/*.sh"} {
		matches, _ := filepath.Glob(filepath.Join(root, pattern))
		if len(matches) > 0 {
			result["shell"] = true
		}
	}
	for _, name := range []string{"tests", "test", "__tests__", "spec"} {
		if dir(name) {
			result["has_tests"] = true
		}
	}
	for _, name := range []string{"README.md", "README.rst", "README.txt", "README"} {
		if file(name) {
			result["has_readme"] = true
		}
	}
	return result
}
