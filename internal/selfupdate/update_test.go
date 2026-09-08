package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func archive(t *testing.T, name, body string, kind byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	size := int64(len(body))
	if kind != tar.TypeReg {
		size = 0
	}
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0755, Size: size, Typeflag: kind}); err != nil {
		t.Fatal(err)
	}
	if size > 0 {
		tw.Write([]byte(body))
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}
func TestReleaseValidation(t *testing.T) {
	data := archive(t, "ash", "binary", tar.TypeReg)
	manifestName := "ashley-1.2.3-darwin-arm64.tar.gz"
	corrupt := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/latest"):
			fmt.Fprint(w, `{"tag_name":"v1.2.3"}`)
		case strings.HasSuffix(r.URL.Path, ".sha256"):
			sum := sha256.Sum256(data)
			fmt.Fprintf(w, "%x  %s\n", sum, manifestName)
		default:
			if corrupt {
				fmt.Fprint(w, "corrupt")
			} else {
				w.Write(data)
			}
		}
	}))
	defer server.Close()
	u := Updater{APIBase: server.URL, DownloadBase: server.URL, Platform: "darwin", Arch: "arm64"}
	if v, err := u.Latest(context.Background()); err != nil || v != "1.2.3" {
		t.Fatal(v, err)
	}
	if binary, err := u.Fetch(context.Background(), "v1.2.3"); err != nil || string(binary) != "binary" {
		t.Fatal(string(binary), err)
	}
	corrupt = true
	if _, err := u.Fetch(context.Background(), "1.2.3"); err == nil {
		t.Fatal("corrupt archive accepted")
	}
	corrupt = false
	for _, c := range []struct {
		name string
		kind byte
	}{{"../ash", tar.TypeReg}, {"ash", tar.TypeSymlink}, {"LICENSE", tar.TypeReg}} {
		data = archive(t, c.name, "test", c.kind)
		if _, err := u.Fetch(context.Background(), "1.2.3"); err == nil {
			t.Fatal(c)
		}
	}
	data = archive(t, "ash", "binary", tar.TypeReg)
	manifestName = "other.tar.gz"
	if _, err := u.Fetch(context.Background(), "1.2.3"); err == nil {
		t.Fatal("wrong manifest accepted")
	}
}
func TestAtomicReplacementAndSourceSymlink(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "launcher")
	destination := filepath.Join(dir, "ash")
	os.WriteFile(source, []byte("old source launcher"), 0700)
	os.Symlink(source, destination)
	if err := Install(context.Background(), destination, "1.2.3", []byte("#!/bin/sh\necho ashley 9.9.9\n")); err == nil {
		t.Fatal("wrong version installed")
	}
	if info, err := os.Lstat(destination); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("old link changed on failure")
	}
	if err := Install(context.Background(), destination, "1.2.3", []byte("#!/bin/sh\necho ashley 1.2.3\n")); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Lstat(destination); info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("source symlink remains")
	}
	if data, _ := os.ReadFile(source); string(data) != "old source launcher" {
		t.Fatal("source checkout modified")
	}
	matches, _ := filepath.Glob(filepath.Join(dir, ".ash-update-*"))
	if len(matches) != 0 {
		t.Fatal("candidate files leaked")
	}
}
func TestVersions(t *testing.T) {
	for _, v := range []string{"1.2.3", "v1.2.3", "1.2.3-rc.1"} {
		if _, err := Version(v); err != nil {
			t.Fatal(v, err)
		}
	}
	for _, v := range []string{"", "main", "../1.2.3", "01.2.3", "1.2.3-evil.0"} {
		if _, err := Version(v); err == nil {
			t.Fatal(v)
		}
	}
}
