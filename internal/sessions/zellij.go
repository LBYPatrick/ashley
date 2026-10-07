package sessions

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// SessionName returns the backend's stable identifier, independently of its label.
func (s Session) SessionName() string {
	if s.Backend == "zellij" {
		return s.ZellijSession
	}
	return s.TmuxSession
}

func (m Manager) zellij(args ...string) (string, error) {
	if m.ZellijRun != nil {
		return m.ZellijRun(args, "")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	binary, err := ZellijBinary()
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("zellij: %w: %s", err, out)
	}
	return string(out), nil
}

// A private config avoids global keybinding changes. Ctrl+B opens Ashley's
// session controls; Ctrl+B then D detaches, S scrolls, B sends literal Ctrl+B.
const zellijConfig = `default_mode "locked"
mouse_mode true
copy_on_select true
copy_clipboard "system"
pane_frames true
pane_frame_style "full"
simplified_ui true
scroll_buffer_size 100000
session_serialization false
on_force_close "detach"
show_startup_tips false
show_release_notes false
support_kitty_keyboard_protocol true
web_server false
keybinds clear-defaults=true {
 locked {
  bind "Ctrl b" { SwitchToMode "Normal"; }
 }
 normal {
  bind "d" { Detach; }
  bind "s" { SwitchToMode "Scroll"; }
  bind "/" { SwitchToMode "EnterSearch"; SearchInput 0; }
  bind "o" { SwitchToMode "Session"; }
  bind "b" { Write 2; SwitchToMode "Locked"; }
  bind "Esc" "Enter" "Ctrl b" { SwitchToMode "Locked"; }
 }
 scroll {
  bind "/" { SwitchToMode "EnterSearch"; SearchInput 0; }
  bind "k" "Up" { ScrollUp; }
  bind "j" "Down" { ScrollDown; }
  bind "PageUp" { PageScrollUp; }
  bind "PageDown" { PageScrollDown; }
  bind "Home" { ScrollToTop; }
  bind "End" { ScrollToBottom; }
  bind "c" { Copy; SwitchToMode "Locked"; }
  bind "q" "Esc" "Enter" { ScrollToBottom; SwitchToMode "Locked"; }
 }
 entersearch {
  bind "Esc" { SearchInput 27; SwitchToMode "Scroll"; }
  bind "Enter" { SwitchToMode "Search"; }
 }
 search {
  bind "n" { Search "down"; }
  bind "p" { Search "up"; }
  bind "/" { SwitchToMode "EnterSearch"; SearchInput 0; }
  bind "c" { SearchToggleOption "CaseSensitivity"; }
  bind "w" { SearchToggleOption "Wrap"; }
  bind "o" { SearchToggleOption "WholeWord"; }
  bind "Up" { ScrollUp; }
  bind "Down" { ScrollDown; }
  bind "PageUp" { PageScrollUp; }
  bind "PageDown" { PageScrollDown; }
  bind "q" "Esc" { ScrollToBottom; SwitchToMode "Locked"; }
 }
 session {
  bind "d" { Detach; }
  bind "Esc" "Enter" { SwitchToMode "Locked"; }
 }
}
load_plugins {}
`

// Native chrome remains non-focusable and follows Zellij's terminal sizing.
// The title supplies the common controls even while agent input is locked.
const zellijLayout = `layout {
 pane focus=true name="Ctrl+B: d Detach / Search s Scroll | Drag Copy | Terminal Paste"
 pane size=1 borderless=true {
  plugin location="zellij:status-bar"
 }
}
`

func (m Manager) startZellij(s Session, args []string) (Session, error) {
	if !validID(s.ID) || s.ZellijSession != "ashley-"+s.ID || len(args) == 0 {
		return Session{}, fmt.Errorf("invalid prepared Zellij session")
	}
	if err := os.MkdirAll(m.Dir, 0700); err != nil {
		return Session{}, err
	}
	configPath := filepath.Join(m.Dir, s.ID+".kdl")
	layoutPath := filepath.Join(m.Dir, s.ID+"-layout.kdl")
	if err := os.WriteFile(layoutPath, []byte(zellijLayout), 0600); err != nil {
		return Session{}, err
	}
	if err := os.WriteFile(configPath, []byte(zellijConfig+"\ndefault_layout "+strconv.Quote(layoutPath)+"\n"), 0600); err != nil {
		os.Remove(layoutPath)
		return Session{}, err
	}
	if err := m.Save(s); err != nil {
		os.Remove(configPath)
		os.Remove(layoutPath)
		return Session{}, err
	}
	// The supervisor opens the log before starting the agent, so even instant
	// startup output is retained without a multiplexer logging race.
	command, _ := ShellCommand(args, s.CWD)
	_, err := m.zellij("--config", configPath, "attach", "--create-background", s.ZellijSession, "--", "bash", "-c", command)
	if err != nil {
		m.zellij("kill-session", s.ZellijSession)
		m.Remove(s, false)
		return Session{}, err
	}
	return s, nil
}
