package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestPresentationWrapsAndLeavesPipesUnstyled(t *testing.T) {
	var out bytes.Buffer
	p := present(&out)
	p.width = 36
	p.heading("History")
	p.table([]string{"ID", "Directory", "Question"}, [][]string{{"123", "/a/very/long/path/to/the/project", strings.Repeat("界", 40)}})
	text := out.String()
	if strings.Contains(text, "\x1b") {
		t.Fatal("ANSI leaked into pipe")
	}
	for _, line := range strings.Split(text, "\n") {
		if ansi.StringWidth(line) > 36 {
			t.Fatalf("line exceeds terminal: %q", line)
		}
	}
	if !strings.Contains(text, "Question") || !strings.Contains(text, "123") {
		t.Fatal("lost fields", text)
	}
}
func TestVendorPromptsAreNotBuffered(t *testing.T) {
	var out bytes.Buffer
	w := &indentedWriter{out: &out, start: true}
	w.Write([]byte("Install? "))
	if out.String() != "    Install? " {
		t.Fatal(out.String())
	}
	w.Write([]byte("yes\nDone\n"))
	if out.String() != "    Install? yes\n    Done\n" {
		t.Fatal(out.String())
	}
}
func TestInstallSummaryAndFullLog(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	for run := range 2 {
		var out, errOut bytes.Buffer
		if err := Run([]string{"install", "--codex", "--skills-only"}, &out, &errOut); err != nil {
			t.Fatal(err, errOut.String())
		}
		text := out.String()
		for _, want := range []string{"Ashley / Install", "Sync skills", "14 skills", "Full log"} {
			if !strings.Contains(text, want) {
				t.Fatal("missing", want, text)
			}
		}
		if strings.Contains(text, "Generated:") || strings.Contains(text, "preserved") {
			t.Fatal("verbose detail in summary", text)
		}
		logs, _ := filepath.Glob(filepath.Join(home, ".ashley/logs/install-*.log"))
		if len(logs) != run+1 {
			t.Fatal("missing per-run log")
		}
		prompt := filepath.Join(home, ".ashley/generated/a-feat/SKILL.md")
		content, _ := os.ReadFile(prompt)
		if string(content) == "local edit" {
			t.Fatal("install retained local edit")
		}
		if run == 0 {
			if err := os.WriteFile(prompt, []byte("local edit"), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	var out bytes.Buffer
	if err := Run([]string{"install", "--codex", "--skills-only", "--verbose"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Generated:") {
		t.Fatal("verbose log unavailable")
	}
	logs, _ := filepath.Glob(filepath.Join(home, ".ashley/logs/install-*.log"))
	for _, log := range logs {
		data, err := os.ReadFile(log)
		if err != nil || strings.Count(string(data), "Generated:") != 14 {
			t.Fatal("incomplete full log", err, string(data))
		}
	}
}
