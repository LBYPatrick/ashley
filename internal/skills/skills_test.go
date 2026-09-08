package skills_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	ashley "github.com/LBYPatrick/ashley"
	"github.com/LBYPatrick/ashley/internal/skills"
)

type fixtures struct {
	Bundled map[string]struct {
		Output       string
		SHA256       string `json:"sha256"`
		PromptSHA256 string `json:"prompt_sha256"`
	}
	Templates []struct {
		Source   string
		Context  map[string]any
		Expected string
	}
	CustomFiles map[string]string `json:"custom_files"`
	Custom      []struct {
		Name     string
		Context  map[string]any
		Output   string
		Expected string
	}
}

func loadFixtures(t *testing.T) fixtures {
	t.Helper()
	data, err := os.ReadFile("../../tests/fixtures/parity/python.json")
	if err != nil {
		t.Fatal(err)
	}
	var f fixtures
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestBundledPythonParity(t *testing.T) {
	f := loadFixtures(t)
	for _, source := range []fs.FS{ashley.Assets, os.DirFS("../..")} {
		catalog := skills.Catalog{Source: source}
		names, err := catalog.Names()
		if err != nil {
			t.Fatal(err)
		}
		if len(names) != len(f.Bundled) {
			t.Fatalf("catalog size: %d, expected %d", len(names), len(f.Bundled))
		}
		for _, name := range names {
			t.Run(name, func(t *testing.T) {
				expected := f.Bundled[name]
				result, err := catalog.Assemble(name, nil)
				if err != nil {
					t.Fatal(err)
				}
				if result.Output != expected.Output {
					t.Errorf("output: %q", result.Output)
				}
				if got := fmt.Sprintf("%x", sha256.Sum256([]byte(result.Content))); got != expected.SHA256 {
					t.Errorf("document differs from Python: %s", got)
				}
				prompt, err := skills.Prompt(result.Content, "add login $HOME `literal`\nnext line", nil)
				if err != nil {
					t.Fatal(err)
				}
				if got := fmt.Sprintf("%x", sha256.Sum256([]byte(prompt))); got != expected.PromptSHA256 {
					t.Errorf("prompt differs from Python: %s", got)
				}
			})
		}
	}
}

func TestTemplatePythonParity(t *testing.T) {
	for _, test := range loadFixtures(t).Templates {
		t.Run(test.Source, func(t *testing.T) {
			got, err := skills.RenderTemplate(test.Source, test.Context)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.Expected {
				t.Errorf("got %q, Python %q", got, test.Expected)
			}
			raw, err := skills.RenderTemplate(test.Source, nil)
			if err != nil || raw != test.Source {
				t.Errorf("generation-time template changed: %q, %v", raw, err)
			}
		})
	}
}

func TestCustomPythonParity(t *testing.T) {
	f := loadFixtures(t)
	source := fstest.MapFS{}
	for name, content := range f.CustomFiles {
		source[name] = &fstest.MapFile{Data: []byte(content)}
	}
	catalog := skills.Catalog{Source: source}
	for _, test := range f.Custom {
		t.Run(test.Name, func(t *testing.T) {
			result, err := catalog.Assemble(test.Name, test.Context)
			if err != nil {
				t.Fatal(err)
			}
			if result.Output != test.Output || result.Content != test.Expected {
				t.Errorf("got:\n%s\nPython:\n%s", result.Content, test.Expected)
			}
		})
	}
}

func TestInvalidDefinition(t *testing.T) {
	for _, test := range []struct{ name, source, problem string }{
		{"cycle", `{"extends":"cycle"}`, "cycle"},
		{"missing", `{"extends":"absent"}`, "absent"},
		{"broken", `{`, "parse skill"},
		{"empty", `{}`, "requires"},
	} {
		t.Run(test.name, func(t *testing.T) {
			catalog := skills.Catalog{Source: fstest.MapFS{"skills/" + test.name + ".jsonc": &fstest.MapFile{Data: []byte(test.source)}}}
			_, err := catalog.Assemble(test.name, nil)
			if err == nil || !strings.Contains(err.Error(), test.problem) {
				t.Fatalf("expected %s error, got %v", test.problem, err)
			}
		})
	}
}

func TestDocumentAliasesAndGeneratedOverride(t *testing.T) {
	catalog := skills.Catalog{Source: ashley.Assets}
	plain, err := catalog.Document("feat")
	if err != nil {
		t.Fatal(err)
	}
	prefixed, err := catalog.Document("a-feat")
	if err != nil {
		t.Fatal(err)
	}
	if plain != prefixed {
		t.Fatal("prefixed skill differed")
	}
	custom := skills.Catalog{Source: fstest.MapFS{"generated/a-custom/SKILL.md": &fstest.MapFile{Data: []byte("edited skill")}}}
	document, err := custom.Document("custom")
	if err != nil || document != "edited skill" {
		t.Fatalf("custom document: %q, %v", document, err)
	}
	if _, err := catalog.Document("nonexistent"); err == nil {
		t.Fatal("expected unknown skill error")
	}
}
