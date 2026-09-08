package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRegistryAndPermissions(t *testing.T) {
	for _, key := range Keys() {
		a := Get(key)
		if a.Key != key || a.Binary != key || !Valid(" "+key+" ") {
			t.Fatal(a)
		}
		for _, mode := range []string{"default", "auto", "dsp", "afk"} {
			flags, actual := PermissionArgs(key, mode == "dsp", mode == "auto", mode == "afk")
			if actual != mode {
				t.Fatal(actual)
			}
			if mode == "default" && len(flags) != 0 {
				t.Fatal(flags)
			}
			if mode == "auto" && !reflect.DeepEqual(flags, a.AutoFlags) {
				t.Fatal(flags)
			}
			if (mode == "dsp" || mode == "afk") && !reflect.DeepEqual(flags, a.DSPFlags) {
				t.Fatal(flags)
			}
		}
		_, mode := PermissionArgs(key, true, true, true)
		if mode != "afk" {
			t.Fatal(mode)
		}
		_, mode = PermissionArgs(key, true, true, false)
		if mode != "dsp" {
			t.Fatal(mode)
		}
		if Trigger(key, "feat", "hello") != a.SkillTrigger+"a-feat hello" || Trigger(key, "a-feat", "") != a.SkillTrigger+"a-feat" {
			t.Fatal("bad trigger")
		}
	}
	a := Get("codex")
	a.DSPFlags[0] = "mutated"
	if Get("codex").DSPFlags[0] == "mutated" {
		t.Fatal("mutated registry")
	}
	if Get("unknown").Key != Default || Valid("unknown") {
		t.Fatal("invalid fallback")
	}
}
func TestPathsAndSelection(t *testing.T) {
	env := map[string]string{}
	getenv := func(key string) string { return env[key] }
	for _, key := range Keys() {
		a := Get(key)
		if got := SkillsDir(key, "/users/test", getenv); got != filepath.Join("/users/test", a.HomeDir, "skills") {
			t.Fatal(got)
		}
		if a.HomeEnv != "" {
			env[a.HomeEnv] = "~/relocated"
			if got := SkillsDir(key, "/users/test", getenv); got != "/users/test/relocated/skills" {
				t.Fatal(got)
			}
			delete(env, a.HomeEnv)
		}
	}
	env["XDG_CONFIG_HOME"] = "/config"
	if got := SkillsDir("opencode", "/users/test", getenv); got != "/config/opencode/skills" {
		t.Fatal(got)
	}
	if key, err := Select("", " CODEX "); err != nil || key != "codex" {
		t.Fatal(key, err)
	}
	if key, err := Select(); err != nil || key != "" {
		t.Fatal(key, err)
	}
	if _, err := Select("claude", "codex"); err == nil {
		t.Fatal("accepted two agents")
	}
	if _, err := Select("unknown"); err == nil {
		t.Fatal("accepted invalid agent")
	}
}

func TestPythonRegistryParity(t *testing.T) {
	data, err := os.ReadFile("../../testdata/parity/python.json")
	if err != nil {
		t.Fatal(err)
	}
	var reference struct {
		Agents []Agent `json:"agents"`
	}
	if err := json.Unmarshal(data, &reference); err != nil {
		t.Fatal(err)
	}
	if len(reference.Agents) != len(Keys()) {
		t.Fatal("missing agent fixture")
	}
	for _, expected := range reference.Agents {
		if !reflect.DeepEqual(Get(expected.Key), expected) {
			t.Fatalf("agent %s differs from Python", expected.Key)
		}
	}
}

func TestNativeBinaryLocations(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "")
	t.Setenv("GROK_BIN_DIR", "")
	for _, c := range []struct{ key, dir string }{{"claude", ".local/bin"}, {"codex", ".local/bin"}, {"grok", ".grok/bin"}, {"opencode", ".opencode/bin"}, {"kilo", ".local/bin"}} {
		path := filepath.Join(home, c.dir, c.key)
		os.MkdirAll(filepath.Dir(path), 0700)
		os.WriteFile(path, []byte("#!/bin/sh\n"), 0700)
		if got, err := FindBinary(c.key); err != nil || got != path {
			t.Fatal(got, err)
		}
	}
	if _, err := FindBinary("nonexistent-command"); err == nil {
		t.Fatal("unknown executable found")
	}
}
