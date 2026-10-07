//go:build !windows

package execution

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoggedAgentHasTTYAndRetainsEarlyOutput(t *testing.T) {
	var out bytes.Buffer
	log := filepath.Join(t.TempDir(), "agent.log")
	cmd := exec.Command("/bin/sh", "-c", "[ -t 0 ] && [ -t 1 ] || exit 99; printf 'first line\\nlast line\\n'; exit 7")
	err := runLogged(cmd, log, strings.NewReader(""), &out)
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 7 {
		t.Fatal(err)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, out.Bytes()) || !strings.Contains(string(data), "first line") || !strings.Contains(string(data), "last line") {
		t.Fatal("lost output", string(data), out.String())
	}
	info, err := os.Stat(log)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal(info, err)
	}
}
