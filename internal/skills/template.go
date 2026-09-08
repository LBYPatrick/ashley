package skills

import (
	"strings"

	"github.com/nikolalohinski/gonja/v2/config"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/nikolalohinski/gonja/v2/loaders"
)

// RenderTemplate uses Jinja syntax and preserves Python's generation-time and
// invalid-syntax behavior. Runtime errors propagate instead of dropping content.
func RenderTemplate(text string, project map[string]any) (string, error) {
	if project == nil || (!strings.Contains(text, "{{") && !strings.Contains(text, "{%")) {
		return text, nil
	}
	loader, err := loaders.NewMemoryLoader(map[string]string{"/template": text})
	if err != nil {
		return "", err
	}
	settings := config.New()
	settings.KeepTrailingNewline = true
	settings.AutoEscape = false
	settings.StrictUndefined = false
	template, err := exec.NewTemplate("/template", settings, loader, templateEnvironment)
	if err != nil {
		return text, nil
	}
	return template.ExecuteToString(exec.NewContext(map[string]any{"project": project}))
}
