package install

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ashley "github.com/LBYPatrick/ashley"
	"github.com/LBYPatrick/ashley/internal/agents"
	"github.com/LBYPatrick/ashley/internal/config"
	"github.com/LBYPatrick/ashley/internal/skills"
)

func TestInstallAllAgentsReplacesLocalEdits(t *testing.T) {
	i := Installer{Home: t.TempDir(), Getenv: func(string) string { return "" }, Catalog: skills.Catalog{Source: ashley.Assets}}
	prefs := config.Store{Dir: filepath.Join(i.Home, ".ashley")}
	prefs.SaveAgent("grok")
	result, err := i.Install(agents.Keys())
	if err != nil {
		t.Fatal(err)
	}
	if result.Installed != 70 || prefs.LoadAgent() != "grok" {
		t.Fatal(result, prefs.LoadAgent())
	}
	for _, key := range agents.Keys() {
		if !i.HasSkills(key) {
			t.Fatal(key)
		}
		data, err := os.ReadFile(filepath.Join(i.directory(key), "a-feat", "SKILL.md"))
		if err != nil || len(data) == 0 {
			t.Fatal(key, err)
		}
	}
	custom := filepath.Join(i.generated(), "a-feat", "SKILL.md")
	os.WriteFile(custom, []byte("customized prompt"), 0600)
	conflict := filepath.Join(i.directory("codex"), "a-debug")
	os.Remove(conflict)
	os.Mkdir(conflict, 0700)
	os.WriteFile(filepath.Join(conflict, "SKILL.md"), []byte("my debug"), 0600)
	if _, err := i.Install(agents.Keys()); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(custom); string(data) == "customized prompt" {
		t.Fatal("local prompt edit retained")
	}
	if data, _ := os.ReadFile(filepath.Join(conflict, "SKILL.md")); string(data) == "my debug" {
		t.Fatal("conflicting skill directory retained")
	}
	result, err = i.Uninstall(nil)
	if err != nil || result.Removed != 70 {
		t.Fatal(result, err)
	}
	if _, err := os.Lstat(conflict); !os.IsNotExist(err) {
		t.Fatal("installed link not removed")
	}
	if _, err := os.Stat(custom); err != nil {
		t.Fatal("local prompt removed")
	}
}
func TestLegacySourceMigrationAndUnrelatedLinks(t *testing.T) {
	i := Installer{Home: t.TempDir(), Getenv: func(string) string { return "" }, Catalog: skills.Catalog{Source: ashley.Assets}}
	directory := i.directory("claude")
	os.MkdirAll(directory, 0700)
	old := filepath.Join(t.TempDir(), "ashley", "generated", "a-feat")
	os.Symlink(old, filepath.Join(directory, "a-feat"))
	unrelated := filepath.Join(t.TempDir(), "custom")
	os.MkdirAll(unrelated, 0700)
	os.WriteFile(filepath.Join(unrelated, "SKILL.md"), []byte("custom"), 0600)
	os.Symlink(unrelated, filepath.Join(directory, "a-commit"))
	if !i.HasSkills("claude") {
		t.Fatal("legacy install not detected")
	}
	if _, err := i.Install(nil); err != nil {
		t.Fatal(err)
	}
	target, err := os.Readlink(filepath.Join(directory, "a-feat"))
	if err != nil || target != filepath.Join(i.generated(), "a-feat") {
		t.Fatal(target, err)
	}
	target, _ = os.Readlink(filepath.Join(directory, "a-commit"))
	if target != filepath.Join(i.generated(), "a-commit") {
		t.Fatal("conflicting link not replaced")
	}
	if data, _ := os.ReadFile(filepath.Join(unrelated, "SKILL.md")); string(data) != "custom" {
		t.Fatal("followed conflicting symlink")
	}
	if _, err := i.Resolve([]string{"unknown"}); err == nil {
		t.Fatal("invalid agent accepted")
	}
	if _, err := i.Uninstall([]string{"unknown"}); err == nil {
		t.Fatal("invalid uninstall accepted")
	}
}

func TestCustomPackageResourcesSurviveInstallAndUpdates(t *testing.T) {
	source := t.TempDir()
	write := func(relative, content string, mode os.FileMode) {
		t.Helper()
		file := filepath.Join(source, relative)
		if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), mode); err != nil {
			t.Fatal(err)
		}
	}
	write("generated/custom/SKILL.md", "Use scripts/check.sh and references/guide.md", 0644)
	write("generated/custom/scripts/check.sh", "#!/bin/sh\necho original\n", 0755)
	write("generated/custom/references/guide.md", "original guide", 0644)
	write("generated/custom/.settings", "hidden resource", 0600)
	i := Installer{Home: t.TempDir(), Getenv: func(string) string { return "" }, Catalog: skills.Catalog{Source: os.DirFS(source)}}
	result, err := i.Install([]string{"codex"})
	if err != nil || result.Installed != 1 {
		t.Fatal(result, err)
	}
	packageDir := filepath.Join(i.directory("codex"), "custom")
	script := filepath.Join(packageDir, "scripts", "check.sh")
	info, err := os.Stat(script)
	if err != nil || info.Mode().Perm() != 0755 {
		t.Fatal(info, err)
	}
	guide := filepath.Join(packageDir, "references", "guide.md")
	if err := os.WriteFile(guide, []byte("local edit"), 0644); err != nil {
		t.Fatal(err)
	}
	write("generated/custom/scripts/check.sh", "#!/bin/sh\necho updated\n", 0755)
	write("generated/custom/references/guide.md", "upstream edit", 0644)
	if _, err := i.Install([]string{"codex"}); err != nil {
		t.Fatal(err)
	}
	// Removing the checkout proves installed resources are standalone.
	if err := os.RemoveAll(source); err != nil {
		t.Fatal(err)
	}
	i.Catalog = skills.Catalog{Source: ashley.Assets}
	if _, err := i.Install([]string{"kilo"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(i.directory("kilo"), "custom", "scripts", "check.sh")); err != nil {
		t.Fatal("custom package unavailable to newly installed agent", err)
	}
	for relative, expected := range map[string]string{
		"SKILL.md":            "Use scripts/check.sh and references/guide.md",
		"scripts/check.sh":    "#!/bin/sh\necho updated\n",
		"references/guide.md": "upstream edit",
		".settings":           "hidden resource",
	} {
		content, err := os.ReadFile(filepath.Join(packageDir, relative))
		if err != nil || string(content) != expected {
			t.Fatalf("%s: %q, %v", relative, content, err)
		}
	}
}

func TestCustomPackageDoesNotWriteThroughResourceSymlink(t *testing.T) {
	source := t.TempDir()
	packageDir := filepath.Join(source, "generated", "custom")
	os.MkdirAll(filepath.Join(packageDir, "references"), 0700)
	os.WriteFile(filepath.Join(packageDir, "SKILL.md"), []byte("custom"), 0600)
	os.WriteFile(filepath.Join(packageDir, "references", "guide.md"), []byte("new"), 0600)
	i := Installer{Home: t.TempDir(), Catalog: skills.Catalog{Source: os.DirFS(source)}}
	os.MkdirAll(filepath.Join(i.generated(), "custom"), 0700)
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "guide.md"), []byte("keep"), 0600)
	os.Symlink(outside, filepath.Join(i.generated(), "custom", "references"))
	if _, err := i.Materialize(); err == nil {
		t.Fatal("accepted a resource directory symlink")
	}
	if content, _ := os.ReadFile(filepath.Join(outside, "guide.md")); string(content) != "keep" {
		t.Fatal("overwrote file outside generated directory")
	}
}

func TestEveryInstallRegeneratesAndLogsAllStages(t *testing.T) {
	var log bytes.Buffer
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	i := Installer{Home: home, Getenv: func(string) string { return "" }, Catalog: skills.Catalog{Source: ashley.Assets}, Log: &log}
	for run := range 2 {
		log.Reset()
		if _, err := i.Install([]string{"claude", "codex"}); err != nil {
			t.Fatal(err)
		}
		text := log.String()
		if strings.Count(text, "Generated:") != 14 || strings.Index(text, "1. Generate skills") >= strings.Index(text, "2. Install skills") {
			t.Fatal("missing generation or wrong stage order", text)
		}
		action := "Installed:"
		if run == 1 {
			action = "Verified link:"
		}
		if strings.Count(text, action) != 28 {
			t.Fatal("incomplete agent log", text)
		}
	}
}
