package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func releaseRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for name, data := range map[string]string{"VERSION": "0.4.0\n", "README.md": "<img src=\"https://img.shields.io/badge/version-0.4.0-blue\" />", "docs/migration/status.json": `{"runtime":false}`} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
func TestReleaseVersionIdentity(t *testing.T) {
	root := releaseRoot(t)
	if err := prepare(root, "1.0.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := check(root, "v1.0.0"); err == nil {
		t.Fatal("incomplete stable release accepted")
	}
	os.WriteFile(filepath.Join(root, "docs/migration/status.json"), []byte(`{"runtime":true}`), 0644)
	if got, err := check(root, "v1.0.0"); err != nil || got != "1.0.0" {
		t.Fatal(got, err)
	}
	if _, err := check(root, "v1.0.1"); err == nil {
		t.Fatal("mismatch accepted")
	}
	if _, err := check(root, "1.0.0"); err == nil {
		t.Fatal("unprefixed tag accepted")
	}
	for _, version := range []string{"0.4.0-alpha.1", "0.4.0-beta.1", "0.4.0-rc.1"} {
		os.WriteFile(filepath.Join(root, "docs/migration/status.json"), []byte(`{"runtime":false}`), 0644)
		for range 2 {
			if err := prepare(root, version); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := check(root, "v"+version); err != nil {
			t.Fatal(err)
		}
	}
}
func TestReleaseRejectsInvalidInputBeforeWriting(t *testing.T) {
	for _, version := range []string{"v1.0.0", "01.0.0", "1.2", "../bad", "1.0.0;echo nope", "1.0.0-unknown.1"} {
		root := releaseRoot(t)
		if err := prepare(root, version); err == nil {
			t.Fatal(version)
		}
		data, _ := os.ReadFile(filepath.Join(root, "VERSION"))
		if string(data) != "0.4.0\n" {
			t.Fatal("partial update")
		}
	}
	root := releaseRoot(t)
	os.WriteFile(filepath.Join(root, "README.md"), []byte("no badge"), 0644)
	if err := prepare(root, "1.0.0"); err == nil {
		t.Fatal("missing badge accepted")
	}
	for _, status := range []string{`{}`, `{"runtime":null}`, `[]`, `{"runtime":"yes"}`} {
		root := releaseRoot(t)
		prepare(root, "1.0.0")
		os.WriteFile(filepath.Join(root, "docs/migration/status.json"), []byte(status), 0644)
		if _, err := check(root, "v1.0.0"); err == nil || !strings.Contains(err.Error(), "status") {
			t.Fatal(err)
		}
	}
}
