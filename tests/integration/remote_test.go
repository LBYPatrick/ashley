//go:build integration && (darwin || linux)

package integration

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/LBYPatrick/ashley/internal/sessions"
	"github.com/creack/pty"
)

func startMoshTerminal(t *testing.T, s *sandbox, args ...string) *terminal {
	t.Helper()
	server, err := exec.LookPath("mosh-server")
	if err != nil {
		t.Skip("mosh-server not installed")
	}
	client, err := exec.LookPath("mosh-client")
	if err != nil {
		t.Skip("mosh-client not installed")
	}
	socket, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := socket.LocalAddr().(*net.UDPAddr).Port
	socket.Close()
	s.env["TERM"] = "xterm-256color"
	argv := []string{"new", "-s", "-i", "127.0.0.1", "-p", strconv.Itoa(port), "--", binary}
	out, err := s.run(server, append(argv, args...)...)
	if err != nil {
		t.Fatal(err, out)
	}
	connection := regexp.MustCompile(`MOSH CONNECT (\d+) (\S+)`).FindStringSubmatch(out)
	if len(connection) != 3 {
		t.Fatal("missing Mosh handshake", out)
	}
	pid := regexp.MustCompile(`pid = (\d+)`).FindStringSubmatch(out)
	if len(pid) == 2 {
		n, _ := strconv.Atoi(pid[1])
		t.Cleanup(func() {
			if p, e := os.FindProcess(n); e == nil {
				p.Kill()
			}
		})
	}
	s.env["MOSH_KEY"] = connection[2]
	s.env["MOSH_PREDICTION_DISPLAY"] = "never"
	return startTerminal(t, s, client, "127.0.0.1", connection[1])
}

func TestMoshWorkspaceAndClipboard(t *testing.T) {
	s := newSandbox(t)
	// Mosh uses its own terminal emulation; verify ordinary input, pasting,
	// command-palette navigation and clipboard delivery through the UDP transport.
	s.env["MOSH_IP"] = "127.0.0.1"
	p := startMoshTerminal(t, s)
	p.waitFor("What would you like")
	p.send("\x10")
	p.waitFor("Commands")
	p.send("New run\r")
	p.until(func() bool { return strings.Contains(sessions.DisplayLog(p.text()), "Start a new conversation") })
	p.send("\x1b[200~mobile task\nkeep this line\x1b[201~")
	p.until(func() bool { return strings.Contains(sessions.DisplayLog(p.text()), "keep this line") })
	p.send("\x13")
	p.until(func() bool { return strings.Contains(sessions.DisplayLog(p.text()), "Search skills") })
	p.send("p")
	p.waitFor("52;c;")
	p.send("\x1b")
	p.until(func() bool { return strings.Contains(sessions.DisplayLog(p.text()), "Start a new conversation") })
	if err := pty.Setsize(p.file, &pty.Winsize{Rows: 24, Cols: 40}); err != nil {
		t.Fatal(err)
	}
	p.send("\x10")
	p.until(func() bool { return strings.Contains(sessions.DisplayLog(p.text()), "Search for commands") })
	p.send("Settings\r")
	p.until(func() bool { return strings.Contains(sessions.DisplayLog(p.text()), "Appearance") })
	p.send("\x03")
	p.exit()
	if strings.Contains(p.text(), "panic:") {
		t.Fatal(fmt.Sprint(p.text()))
	}
}

func TestSSHWorkspaceTransport(t *testing.T) {
	sshd, err := exec.LookPath("sshd")
	if err != nil {
		sshd = "/usr/sbin/sshd"
		if _, err = os.Stat(sshd); err != nil {
			t.Skip("sshd not installed")
		}
	}
	ssh, err := exec.LookPath("ssh")
	if err != nil {
		t.Skip("ssh not installed")
	}
	keygen, err := exec.LookPath("ssh-keygen")
	if err != nil {
		t.Skip("ssh-keygen not installed")
	}
	s := newSandbox(t)
	for _, name := range []string{"host", "client"} {
		s.must(keygen, "-q", "-t", "ed25519", "-N", "", "-f", filepath.Join(s.home, name))
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	config := fmt.Sprintf("Port %d\nListenAddress 127.0.0.1\nHostKey %s/host\nPidFile %s/sshd.pid\nAuthorizedKeysFile %s/client.pub\nStrictModes no\nPasswordAuthentication no\nKbdInteractiveAuthentication no\nUsePAM no\n", port, s.home, s.home, s.home)
	configPath := filepath.Join(s.home, "sshd_config")
	write(t, configPath, config, 0600)
	var log bytes.Buffer
	daemon := exec.Command(sshd, "-D", "-e", "-f", configPath)
	daemon.Stderr = &log
	if err = daemon.Start(); err != nil {
		t.Skipf("cannot start isolated sshd: %v", err)
	}
	finished := make(chan error, 1)
	go func() { finished <- daemon.Wait() }()
	t.Cleanup(func() {
		daemon.Process.Kill()
		select {
		case <-finished:
		case <-time.After(time.Second):
		}
	})
	deadline := time.Now().Add(3 * time.Second)
	for {
		c, e := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
		if e == nil {
			c.Close()
			break
		}
		select {
		case e := <-finished:
			t.Skipf("isolated sshd unavailable: %v: %s", e, log.String())
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("sshd did not become ready")
		}
		time.Sleep(20 * time.Millisecond)
	}
	command := "cd " + sessions.Quote(s.home) + " && env"
	for _, key := range []string{"HOME", "PATH", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "ZELLIJ_SOCKET_DIR"} {
		command += " " + sessions.Quote(key+"="+s.env[key])
	}
	command += " " + sessions.Quote(binary)
	p := startTerminal(t, s, ssh, "-tt", "-F", "/dev/null", "-p", strconv.Itoa(port), "-i", filepath.Join(s.home, "client"), "-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes", "-o", "StrictHostKeyChecking=accept-new", "-o", "UserKnownHostsFile="+filepath.Join(s.home, "known_hosts"), "127.0.0.1", command)
	p.waitFor("What would you like")
	p.send("\x1b[50;5u")
	p.waitFor("Start a new conversation")
	p.send("\x1b[200~SSH task\nsecond line\x1b[201~")
	p.waitFor("second line")
	p.send("\x03")
	p.exit()
}
