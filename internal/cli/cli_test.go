package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ashley "github.com/LBYPatrick/ashley"
	"github.com/LBYPatrick/ashley/internal/cli"
	"github.com/LBYPatrick/ashley/internal/skills"
)

func invoke(args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	err := cli.Run(args, &stdout, &stderr)
	return stdout.String(), err
}

func TestReadOnlyCommands(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"--version"}, "ashley " + ashley.Version()},
		{[]string{"version"}, "ashley " + ashley.Version()},
		{[]string{"list"}, "coding"},
		{[]string{"--help"}, "coding-agent skill launcher"},
		{[]string{"prompt", "feat", "add", "login"}, "<command-args>\nadd login\n</command-args>"},
		{[]string{"prompt", "a-feat"}, "Workflow:"},
	} {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			output, err := invoke(test.args...)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output, test.want) {
				t.Fatalf("output %q missing %q", output, test.want)
			}
		})
	}
}

func TestGenerateWithoutPythonOrSourceCheckout(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	root := t.TempDir()
	output, err := invoke("generate", "--output", root)
	if err != nil {
		t.Fatal(err)
	}
	catalog := skills.Catalog{Source: ashley.Assets}
	names, err := catalog.Names()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		expected, err := catalog.Assemble(name, nil)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(root, expected.Output))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != expected.Content {
			t.Fatalf("generated %s differed", name)
		}
	}
	if !strings.Contains(output, "Generated:") {
		t.Fatal(output)
	}
}

func TestCustomRepository(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"skills/custom.jsonc":  `{name:'a-custom',description:'Custom',output:'generated/a-custom/SKILL.md',components:['components/custom.md']}`,
		"components/custom.md": "{% if project.go %}Use Go{% endif %}\n",
		"go.mod":               "module example.test/demo",
	}
	for name, content := range files {
		dest := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dest, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := invoke("--root", root, "generate"); err != nil {
		t.Fatal(err)
	}
	raw, err := invoke("--root", root, "prompt", "custom")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, "{% if project.go %}") {
		t.Fatal("generation rendered a template without context")
	}
	rendered, err := invoke("--root", root, "prompt", "--project", root, "custom", "task")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rendered, "{%") || !strings.Contains(rendered, "Use Go") {
		t.Fatal(rendered)
	}
	detection, err := invoke("detect", root)
	if err != nil || !strings.Contains(detection, `"go": true`) {
		t.Fatalf("%s, %v", detection, err)
	}
}

func TestUsageErrors(t *testing.T) {
	for _, args := range [][]string{{"run", "feat"}, {"prompt"}, {"prompt", "absent"}, {"list", "extra"}, {"generate", "extra"}, {"--unknown"}, {"prompt", "--project", "/nonexistent-ashley-project", "feat"}} {
		if _, err := invoke(args...); err == nil {
			t.Errorf("expected error for %v", args)
		}
	}
}

func TestGenerateRejectsEscapingOutput(t *testing.T) {
	for _, output := range []string{"../escaped.md", "/tmp/escaped.md"} {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "skills"), 0755); err != nil {
			t.Fatal(err)
		}
		def := `{"name":"a-bad","description":"bad","output":"` + output + `"}`
		if err := os.WriteFile(filepath.Join(root, "skills/bad.jsonc"), []byte(def), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := invoke("--root", root, "generate"); err == nil {
			t.Errorf("accepted escaping output %s", output)
		}
	}
}

func TestGeneratedSymlinkCannotEscapeOutputRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "generated")); err != nil {
		t.Fatal(err)
	}
	if _, err := invoke("generate", "--output", root); err == nil {
		t.Fatal("followed symlink outside output root")
	}
	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("modified directory outside output root")
	}
}
