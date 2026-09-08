package selfupdate

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDevelopmentUpdateAndDirtyCheckoutProtection(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test"), 0600)
	os.WriteFile(filepath.Join(root, "VERSION"), []byte("1.2.3"), 0600)
	tools := t.TempDir()
	t.Setenv("PATH", tools)
	git := filepath.Join(tools, "git")
	os.WriteFile(git, []byte("#!/bin/sh\nif [ \"$3\" = status ]; then printf '%s' \"$TEST_DIRTY\"; fi\n"), 0700)
	os.WriteFile(filepath.Join(tools, "go"), []byte("#!/bin/sh\nwhile [ \"$1\" != -o ]; do shift; done\nshift\nprintf '#!/bin/sh\\necho ashley 1.2.3\\n' > \"$1\"\n"), 0700)
	destination := filepath.Join(t.TempDir(), "ash")
	os.WriteFile(destination, []byte("old binary"), 0700)
	t.Setenv("TEST_DIRTY", " M user-file")
	if err := SourceUpdate(context.Background(), root, destination, "feat/work", io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "commit or stash") {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(destination); string(data) != "old binary" {
		t.Fatal("dirty checkout update modified binary")
	}
	t.Setenv("TEST_DIRTY", "")
	if err := SourceUpdate(context.Background(), root, destination, "feat/work", io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(destination); !strings.Contains(string(data), "1.2.3") {
		t.Fatal(string(data))
	}
}
