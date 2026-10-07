package sessions

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestZellijLifecycleCommandsAndPrivateConfig(t *testing.T) {
	var calls [][]string
	live := true
	m := Manager{Dir: t.TempDir(), ZellijRun: func(args []string, _ string) (string, error) {
		calls = append(calls, append([]string{}, args...))
		if args[0] == "list-sessions" && live {
			return "ashley-fixture\n", nil
		}
		if args[0] == "kill-session" {
			live = false
		}
		return "", nil
	}}
	s := Session{ID: "fixture", Backend: "zellij", ZellijSession: "ashley-fixture", CWD: "/tmp/a project", LogFile: filepath.Join(m.Dir, "fixture.log")}
	got, err := m.Start(s, []string{"/bin/echo", "literal $HOME; 'text'"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Backend != "zellij" || !m.Alive(s) {
		t.Fatal(got)
	}
	if !reflect.DeepEqual(calls[0][:5], []string{"--config", filepath.Join(m.Dir, "fixture.kdl"), "attach", "--create-background", "ashley-fixture"}) {
		t.Fatal(calls)
	}
	cfg, err := os.ReadFile(filepath.Join(m.Dir, "fixture.kdl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"default_mode \"locked\"", "on_force_close \"detach\"", "copy_on_select true", "session_serialization false", "web_server false"} {
		if !strings.Contains(string(cfg), v) {
			t.Fatal(v)
		}
	}
	loaded, err := m.Load(s.ID)
	if err != nil || loaded.SessionName() != s.ZellijSession {
		t.Fatal(loaded, err)
	}
	if err = m.Kill(s); err != nil {
		t.Fatal(err)
	}
	if m.Alive(s) {
		t.Fatal("still alive")
	}
	if _, err = os.Stat(filepath.Join(m.Dir, "fixture.kdl")); !os.IsNotExist(err) {
		t.Fatal("config leaked", err)
	}
}
func TestVerifiedZellijArchive(t *testing.T) {
	for _, name := range []string{"zellij", "../../zellij"} {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		tw.WriteHeader(&tar.Header{Name: name, Mode: 0755, Size: 6})
		tw.Write([]byte("binary"))
		tw.Close()
		gz.Close()
		sum := fmt.Sprintf("%x", sha256.Sum256(buf.Bytes()))
		got, err := unpackZellij(buf.Bytes(), sum)
		if name == "zellij" && (err != nil || string(got) != "binary") {
			t.Fatal(got, err)
		}
		if name != "zellij" && err == nil {
			t.Fatal("unsafe archive accepted")
		}
		if _, err = unpackZellij(buf.Bytes(), strings.Repeat("0", 64)); err == nil {
			t.Fatal("checksum ignored")
		}
	}
}

func TestEnsureZellijDetectsInstalledWithoutNetwork(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "")
	bin := filepath.Join(home, ".local/bin")
	if err := os.MkdirAll(bin, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "zellij"), []byte("#!/bin/sh\necho 'zellij 0.45.1'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := EnsureZellij(&out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Zellij ready") {
		t.Fatal(out.String())
	}
}

func TestZellijStarterConfigPreservesUserSettings(t *testing.T) {
	for _, override := range []string{"default", "xdg", "directory", "file"} {
		t.Run(override, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", "")
			t.Setenv("ZELLIJ_CONFIG_DIR", "")
			t.Setenv("ZELLIJ_CONFIG_FILE", "")
			want := filepath.Join(home, ".config", "zellij", "config.kdl")
			switch override {
			case "xdg":
				t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
				want = filepath.Join(home, "xdg", "zellij", "config.kdl")
			case "directory":
				t.Setenv("ZELLIJ_CONFIG_DIR", filepath.Join(home, "custom"))
				want = filepath.Join(home, "custom", "config.kdl")
			case "file":
				want = filepath.Join(home, "custom", "mine.kdl")
				t.Setenv("ZELLIJ_CONFIG_FILE", want)
			}
			var out bytes.Buffer
			if err := ensureZellijConfig(&out); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(want)
			if err != nil || !strings.Contains(string(data), "default_layout") || !strings.Contains(string(data), "SearchInput 0") {
				t.Fatal(string(data), err)
			}
			layouts, err := filepath.Glob(filepath.Join(filepath.Dir(want), "ashley-layout-*.kdl"))
			if err != nil || len(layouts) != 1 {
				t.Fatal(layouts, err)
			}
			data, err = os.ReadFile(layouts[0])
			if err != nil || !strings.Contains(string(data), "zellij:status-bar") {
				t.Fatal(string(data), err)
			}
			const customized = "// user's own config\nmouse_mode false\n"
			if err := os.WriteFile(want, []byte(customized), 0600); err != nil {
				t.Fatal(err)
			}
			if err := ensureZellijConfig(&out); err != nil {
				t.Fatal(err)
			}
			data, err = os.ReadFile(want)
			if err != nil || string(data) != customized {
				t.Fatal("replaced user config", string(data), err)
			}
			layouts, _ = filepath.Glob(filepath.Join(filepath.Dir(want), "ashley-layout-*.kdl"))
			if len(layouts) != 1 {
				t.Fatal("repeat setup leaked layouts", layouts)
			}
		})
	}
}
