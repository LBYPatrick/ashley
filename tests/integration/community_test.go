//go:build integration && (darwin || linux)

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedCommunityBundleLifecycle(t *testing.T) {
	s := newSandbox(t)
	communityFixtures(t, s)
	agentDir := filepath.Join(s.home, ".claude", "skills")
	unmanaged := filepath.Join(agentDir, "modern-web-guidance", "SKILL.md")
	write(t, unmanaged, "Independent skill\n", 0600)

	output := s.must(binary, "__community-bundles", "modern-web", "find-skills")
	requireContains(t, output, "Installing Modern Web Guidance")
	requireContains(t, output, "Installing Find Skills")
	if strings.Contains(output, "[Y/n]") || strings.Contains(output, "[y/N]") {
		t.Fatal("confirmed selection asked for confirmation again", output)
	}
	managed := filepath.Join(agentDir, "skills")
	if _, err := os.Readlink(managed); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"modern-web", "find-skills"} {
		if !exists(filepath.Join(s.home, ".ashley", "community", id, "bundle.json")) {
			t.Fatalf("missing marker for %s", id)
		}
	}

	output = s.must(binary, "__community-bundles", "find-skills")
	requireContains(t, output, "Removing Modern Web Guidance")
	if strings.Contains(output, "Installing Find Skills") {
		t.Fatal("unchanged bundle was downloaded again", output)
	}
	s.must(binary, "__community-bundles")
	if _, err := os.Lstat(managed); !os.IsNotExist(err) {
		t.Fatalf("managed skill remains: %v", err)
	}
	if data := read(t, unmanaged); data != "Independent skill\n" {
		t.Fatalf("independent skill changed: %q", data)
	}
	if got := strings.Count(read(t, filepath.Join(s.home, "community-calls")), "\n"); got != 2 {
		t.Fatalf("expected one vendor call per installed bundle, got %d", got)
	}
}
