package community

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestApplyPreservesIndependentSkillsAndTracksManagedChanges(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("community installation supports macOS and Linux")
	}
	home := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
	claude := filepath.Join(home, ".claude", "skills")
	codex := filepath.Join(home, ".codex", "skills")
	prior := filepath.Join(claude, "emil")
	writeSkill(t, prior, "Independent copy")
	var fetched []string
	fetch := func(bundle Bundle, stage string) error {
		fetched = append(fetched, bundle.ID)
		writeSkill(t, filepath.Join(stage, ".agents", "skills", bundle.ID), bundle.Label)
		return nil
	}
	var output bytes.Buffer
	apply := func(keys, desired []string) {
		t.Helper()
		if err := Apply(home, keys, desired, fetch, &output); err != nil {
			t.Fatal(err)
		}
	}
	apply([]string{"claude"}, []string{"emil", "find-skills"})
	if !strings.Contains(output.String(), "Keeping existing skill") {
		t.Fatal("pre-existing skill collision should be visible")
	}
	check := func(id string, want bool) {
		t.Helper()
		installed, err := Installed(home)
		if err != nil || installed[id] != want {
			t.Fatalf("installed %s = %v, want %v (error %v)", id, installed[id], want, err)
		}
	}
	check("emil", true)
	check("find-skills", true)
	check("modern-web", false)
	if err := os.Remove(filepath.Join(claude, "find-skills")); err != nil {
		t.Fatal(err)
	}
	check("find-skills", false)
	apply([]string{"claude", "codex"}, []string{"emil", "find-skills"})
	check("find-skills", true)
	if got := strings.Join(fetched, ","); got != "emil,find-skills" {
		t.Fatalf("installed again for a new agent or repair: %s", got)
	}
	for _, agent := range []string{claude, codex} {
		if _, err := os.Readlink(filepath.Join(agent, "find-skills")); err != nil {
			t.Fatal(err)
		}
	}
	// Unchecking removes the links for both original targets even if the current target list changed.
	apply([]string{"codex"}, nil)
	for _, name := range []string{filepath.Join(claude, "find-skills"), filepath.Join(codex, "find-skills"), filepath.Join(codex, "emil")} {
		if _, err := os.Lstat(name); !os.IsNotExist(err) {
			t.Fatalf("managed skill remained: %s", name)
		}
	}
	if data, err := os.ReadFile(filepath.Join(prior, "SKILL.md")); err != nil || string(data) != "Independent copy" {
		t.Fatalf("independent skill changed: %s (%v)", data, err)
	}
	check("emil", false)
	check("find-skills", false)
}

func TestApplyFailedFetchDoesNotClaimInstallation(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("community installation supports macOS and Linux")
	}
	home := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	failure := errors.New("fixture download failed")
	var output bytes.Buffer
	err := Apply(home, []string{"claude"}, []string{"emil", "modern-web"}, func(bundle Bundle, stage string) error {
		writeSkill(t, filepath.Join(stage, ".agents", "skills", bundle.ID), bundle.Label)
		if bundle.ID == "modern-web" {
			return failure
		}
		return nil
	}, &output)
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	installed, err := Installed(home)
	if err != nil || !installed["emil"] || installed["modern-web"] {
		t.Fatalf("invalid status after download failure: %v (%v)", installed, err)
	}
	entries, err := os.ReadDir(root(home))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".modern-web-") {
			t.Fatal("temporary download was retained")
		}
	}
}

func writeSkill(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
