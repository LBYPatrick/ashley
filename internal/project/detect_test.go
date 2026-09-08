package project_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/LBYPatrick/ashley/internal/project"
)

func TestPythonParity(t *testing.T) {
	data, err := os.ReadFile("../../testdata/parity/python.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct {
		Projects []struct {
			Files    map[string]string
			Expected map[string]any
		}
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, test := range fixtures.Projects {
		root := filepath.Join(t.TempDir(), "project")
		if err := os.MkdirAll(root, 0755); err != nil {
			t.Fatal(err)
		}
		for name, content := range test.Files {
			dest := filepath.Join(root, name)
			if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dest, []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
		}
		if got := project.Detect(root); !reflect.DeepEqual(got, test.Expected) {
			t.Errorf("got: %#v\nPython: %#v", got, test.Expected)
		}
	}
}
