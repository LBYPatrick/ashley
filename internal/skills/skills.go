// Package skills loads JSON5 skill definitions and assembles portable prompts.
package skills

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/titanous/json5"
)

// Definition is the existing JSONC skill format. Nil slices mean absent keys;
// an explicit empty list replaces inherited components or resources.
type Definition struct {
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	Output           string    `json:"output"`
	Preamble         string    `json:"preamble"`
	Epilogue         string    `json:"epilogue"`
	Extends          string    `json:"extends"`
	Components       []string  `json:"components"`
	Resources        []string  `json:"resources"`
	ComponentsAppend []string  `json:"components+"`
	ResourcesAppend  []string  `json:"resources+"`
	Workflow         *Workflow `json:"workflow"`
	Checklist        []string  `json:"checklist"`
}

// Workflow describes the ordered actions and validations for a skill.
type Workflow struct {
	Name  string `json:"name"`
	Steps []Step `json:"steps"`
}

// Step is one named workflow stage.
type Step struct {
	Name         string   `json:"name"`
	Instructions []string `json:"instructions"`
	Validation   string   `json:"validation"`
}

// Result is a generated document and its path relative to the source root.
type Result struct {
	Output  string
	Content string
}

// Catalog reads an embedded or on-disk Ashley repository without modifying it.
type Catalog struct{ Source fs.FS }

// Names returns the sorted filename stems of all available skill definitions.
func (c Catalog) Names() ([]string, error) {
	files, err := fs.Glob(c.Source, "skills/*.jsonc")
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no skill definitions found in skills/")
	}
	names := make([]string, len(files))
	for i, file := range files {
		names[i] = strings.TrimSuffix(path.Base(file), ".jsonc")
	}
	sort.Strings(names)
	return names, nil
}

// Load parses a skill and resolves chained component/resource inheritance.
func (c Catalog) Load(name string) (Definition, error) {
	return c.load(name, make(map[string]bool))
}

func (c Catalog) load(name string, visiting map[string]bool) (Definition, error) {
	var def Definition
	if !fs.ValidPath(name) || strings.Contains(name, "/") {
		return def, fmt.Errorf("invalid skill name %q", name)
	}
	if visiting[name] {
		return def, fmt.Errorf("skill inheritance cycle at %q", name)
	}
	visiting[name] = true
	defer delete(visiting, name)
	data, err := fs.ReadFile(c.Source, "skills/"+name+".jsonc")
	if err != nil {
		return def, fmt.Errorf("load skill %q: %w", name, err)
	}
	if err = json5.Unmarshal(data, &def); err != nil {
		return def, fmt.Errorf("parse skill %q: %w", name, err)
	}
	if def.Extends != "" {
		base, err := c.load(def.Extends, visiting)
		if err != nil {
			return def, err
		}
		if def.Components == nil {
			def.Components = append(base.Components, def.ComponentsAppend...)
		}
		if def.Resources == nil {
			def.Resources = append(base.Resources, def.ResourcesAppend...)
		}
		def.Extends = ""
		def.ComponentsAppend, def.ResourcesAppend = nil, nil
	}
	return def, nil
}

// Assemble renders a skill. A nil project leaves Jinja expressions untouched.
func (c Catalog) Assemble(name string, project map[string]any) (Result, error) {
	def, err := c.Load(name)
	if err != nil {
		return Result{}, err
	}
	if def.Name == "" || def.Description == "" || def.Output == "" {
		return Result{}, fmt.Errorf("skill %q requires name, description and output", name)
	}
	parts := []string{"---", "name: " + def.Name, "description: " + def.Description, "---\n"}
	if def.Preamble != "" {
		parts = append(parts, def.Preamble+"\n")
	}
	if def.Workflow != nil {
		parts = append(parts, renderWorkflow(*def.Workflow))
	}
	parts = append(parts, "---\n")
	for _, component := range def.Components {
		var matches []string
		if strings.ContainsAny(component, "*?[") {
			matches, err = doublestar.Glob(c.Source, component)
			if err != nil {
				return Result{}, fmt.Errorf("component pattern %q: %w", component, err)
			}
			sort.Strings(matches)
			if len(matches) == 0 {
				parts = append(parts, fmt.Sprintf("<!-- WARNING: No files matched pattern: %s -->\n", component))
			}
		} else if info, err := fs.Stat(c.Source, component); err == nil && !info.IsDir() {
			matches = []string{component}
		} else {
			parts = append(parts, fmt.Sprintf("<!-- WARNING: Component not found: %s -->\n", component))
		}
		for _, match := range matches {
			data, err := fs.ReadFile(c.Source, match)
			if err != nil {
				return Result{}, fmt.Errorf("read component %q: %w", match, err)
			}
			text, err := RenderTemplate(string(data), project)
			if err != nil {
				return Result{}, fmt.Errorf("render component %q: %w", match, err)
			}
			parts = append(parts, text, "\n")
		}
	}
	var resources []string
	for _, resource := range def.Resources {
		if info, err := fs.Stat(c.Source, resource); err == nil && !info.IsDir() {
			resources = append(resources, resource)
		} else {
			parts = append(parts, fmt.Sprintf("<!-- WARNING: Resource not found: %s -->\n", resource))
		}
	}
	if len(resources) > 0 {
		parts = append(parts, "---\n", "## Reference Resources\n", "Use these code templates as reference when implementing:\n")
		for _, resource := range resources {
			data, err := fs.ReadFile(c.Source, resource)
			if err != nil {
				return Result{}, err
			}
			language := map[string]string{".py": "python", ".ts": "typescript", ".tsx": "typescript", ".js": "javascript", ".jsx": "javascript", ".json": "json", ".sh": "bash"}[path.Ext(resource)]
			parts = append(parts, fmt.Sprintf("### `%s`\n\n```%s\n%s\n```\n", path.Base(resource), language, strings.TrimRightFunc(string(data), unicode.IsSpace)))
		}
	}
	if len(def.Checklist) > 0 {
		lines := []string{"## Completion Checklist\n"}
		for _, item := range def.Checklist {
			lines = append(lines, "- [ ] "+item)
		}
		parts = append(parts, "---\n", strings.Join(lines, "\n"), "")
	} else if def.Epilogue != "" {
		parts = append(parts, "---\n", def.Epilogue+"\n")
	}
	return Result{Output: def.Output, Content: strings.Join(parts, "\n")}, nil
}

func renderWorkflow(w Workflow) string {
	lines := []string{
		"## Workflow: " + w.Name + "\n",
		"**IMPORTANT — Sequential Workflow Orchestration Rules:**",
		"- Execute steps in numbered order. Never jump ahead.",
		"- After completing each step, validate it succeeded before proceeding.",
		"- If validation fails, fix the issue in the current step before moving on. Retry up to 2 times.",
		"- If a step fails after retries, report to the user: what failed, what you tried, and suggested next steps.",
		"- On unrecoverable failure, rollback partial changes so the codebase is left clean.",
		"- Announce each step before starting (e.g., \"Step 1: Analyzing the codebase...\").\n",
	}
	for i, step := range w.Steps {
		lines = append(lines, fmt.Sprintf("### Step %d: %s", i+1, step.Name))
		for j, instruction := range step.Instructions {
			lines = append(lines, fmt.Sprintf("%d. %s", j+1, instruction))
		}
		lines = append(lines, "- **Validation:** "+step.Validation+"\n")
	}
	return strings.Join(lines, "\n")
}

var frontmatter = regexp.MustCompile(`(?s)^---\n.*?\n---\n*`)

// Prompt strips frontmatter and appends the question using Ashley's existing format.
func Prompt(document, question string, project map[string]any) (string, error) {
	body := frontmatter.ReplaceAllString(document, "")
	var err error
	if len(project) > 0 {
		body, err = RenderTemplate(body, project)
	}
	if err != nil {
		return "", err
	}
	body = strings.TrimSpace(body)
	if question != "" {
		body += "\n\n<command-args>\n" + question + "\n</command-args>"
	}
	return body, nil
}

// Document reads an existing generated document first, then assembles from source.
// The source fallback lets a standalone binary work without a generated/ checkout.
func (c Catalog) Document(name string) (string, error) {
	if !fs.ValidPath(name) || strings.Contains(name, "/") {
		return "", fmt.Errorf("invalid skill name %q", name)
	}
	for _, candidate := range []string{"a-" + name, name} {
		data, err := fs.ReadFile(c.Source, "generated/"+candidate+"/SKILL.md")
		if err == nil {
			return string(data), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
	}
	stem := strings.TrimPrefix(name, "a-")
	if _, err := fs.Stat(c.Source, "skills/"+stem+".jsonc"); err == nil {
		result, err := c.Assemble(stem, nil)
		return result.Content, err
	}
	return "", fmt.Errorf("skill %q not found; use 'ash list' to see available skills", name)
}
