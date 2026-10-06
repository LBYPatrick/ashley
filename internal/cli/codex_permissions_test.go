package cli

import (
	"slices"
	"strings"
	"testing"

	"github.com/LBYPatrick/ashley/internal/invocation"
)

func TestCodexLaunchPermissions(t *testing.T) {
	for _, tc := range []struct {
		flag string
		mode string
		want []string
	}{
		{"-dsp", "dsp", []string{"--dangerously-bypass-approvals-and-sandbox", "--sandbox", "danger-full-access"}},
		{"--leon", "afk", []string{"--dangerously-bypass-approvals-and-sandbox", "--sandbox", "danger-full-access"}},
		{"--afk", "afk", []string{"--dangerously-bypass-approvals-and-sandbox", "--sandbox", "danger-full-access"}},
		{"--auto", "auto", []string{"--sandbox", "workspace-write", "--ask-for-approval", "never"}},
		{"--normal", "default", nil},
	} {
		t.Run(tc.flag, func(t *testing.T) {
			o, _, err := runOptions([]string{"raw", "--codex", tc.flag})
			if err != nil {
				t.Fatal(err)
			}
			b := invocation.Builder{Home: t.TempDir(), LookPath: func(string) (string, error) { return "/bin/codex", nil }}
			v, err := b.Build(o)
			if err != nil {
				t.Fatal(err)
			}
			defer v.Cleanup()
			args := v.Args[1:]
			if tc.mode == "afk" {
				if len(args) == 0 || !strings.Contains(args[len(args)-1], "AFK Mode") {
					t.Fatal("missing AFK instructions", args)
				}
				args = args[:len(args)-1]
			}
			if v.Permission != tc.mode || !slices.Equal(args, tc.want) {
				t.Fatalf("got mode %q args %q, want %q %q", v.Permission, args, tc.mode, tc.want)
			}
		})
	}
}
