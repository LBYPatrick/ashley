package upgrade

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBrewPackageNeverUsesScriptBasenames(t *testing.T) {
	for _, c := range []struct {
		path, want string
		cask       bool
	}{{"/opt/homebrew/Cellar/codex/1.0/bin/codex", "codex", false}, {"/opt/homebrew/Caskroom/claude-code/1.0/claude", "claude-code", true}, {"/opt/homebrew/lib/node_modules/@openai/codex/bin/codex.js", "", false}, {"/opt/homebrew/bin/codex.js", "", false}, {"/other/Cellar/codex/1/bin/codex", "", false}} {
		name, cask := BrewPackage(c.path, "/opt/homebrew")
		if name != c.want || cask != c.cask {
			t.Fatal(c, name, cask)
		}
	}
}

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestUpgradeFallbackAndNativeInstall(t *testing.T) {
	var calls [][]string
	client := &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("vendor script"))}, nil
	})}
	m := Manager{Client: client, LookPath: func(name string) (string, error) {
		if name == "brew" {
			return "", os.ErrNotExist
		}
		return "/bin/" + name, nil
	}, Run: func(ctx context.Context, args []string, input io.Reader) (string, error) {
		calls = append(calls, args)
		if len(args) > 1 && args[1] == "--version" {
			return "version 1\nmore", nil
		}
		if args[0] == "/bin/codex" {
			return "", fmt.Errorf("native update refused")
		}
		if input != nil {
			data, _ := io.ReadAll(input)
			if string(data) != "vendor script" {
				t.Fatal(string(data))
			}
		}
		return "", nil
	}}
	if err := m.Upgrade(context.Background(), "codex"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls[len(calls)-1], []string{"sh", "-s"}) {
		t.Fatal(calls)
	}
	if err := m.NativeInstall(context.Background(), "kilo"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls[len(calls)-1], []string{"npm", "install", "-g", "@kilocode/cli"}) {
		t.Fatal(calls)
	}
	count := len(calls)
	if err := m.Ensure(context.Background(), "claude"); err != nil || len(calls) != count {
		t.Fatal("reinstalled existing agent")
	}
	client.Transport = transport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 503, Status: "503 unavailable", Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	if err := m.NativeInstall(context.Background(), "grok"); err == nil {
		t.Fatal("download failure ignored")
	}
}
func TestHomebrewUpgradeIsExclusive(t *testing.T) {
	prefix := t.TempDir()
	path := filepath.Join(prefix, "Caskroom", "codex", "1", "codex")
	os.MkdirAll(filepath.Dir(path), 0700)
	os.WriteFile(path, []byte("binary"), 0700)
	var calls [][]string
	m := Manager{LookPath: func(name string) (string, error) {
		if name == "brew" {
			return "/bin/brew", nil
		}
		return path, nil
	}, Run: func(ctx context.Context, args []string, input io.Reader) (string, error) {
		calls = append(calls, args)
		if args[1] == "--prefix" {
			return prefix, nil
		}
		if args[1] == "upgrade" {
			return "", fmt.Errorf("brew failed")
		}
		return "1.0", nil
	}}
	if err := m.Upgrade(context.Background(), "codex"); err == nil {
		t.Fatal("brew failure ignored")
	}
	if !reflect.DeepEqual(calls[len(calls)-1], []string{"brew", "upgrade", "--cask", "codex"}) {
		t.Fatal(calls)
	}
}
