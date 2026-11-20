package tui

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"strings"

	"github.com/LBYPatrick/ashley/internal/community"
	"github.com/LBYPatrick/ashley/internal/sessions"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

const communityScreen = "community"

type communityManager struct {
	installed, selected map[string]bool
	cursor              int
	busy                bool
	cancel              context.CancelFunc
	stateErr, applyErr  error
	output              string
	showLog             bool
	logOffset           int
}

type communityResult struct {
	installed          map[string]bool
	stateErr, applyErr error
	output             string
}

func (m *Model) openCommunity() tea.Cmd {
	m.open(communityScreen)
	if !m.community.busy {
		installed, err := community.Installed(m.options.Home)
		m.community = communityManager{installed: cloneBundleSelection(installed), selected: cloneBundleSelection(installed), stateErr: err}
	}
	return nil
}

func cloneBundleSelection(installed map[string]bool) map[string]bool {
	if installed == nil {
		return make(map[string]bool)
	}
	selected := make(map[string]bool)
	for id, present := range installed {
		if present {
			selected[id] = true
		}
	}
	return selected
}

func (m *Model) communityFinished(result communityResult) {
	c := &m.community
	c.busy, c.cancel = false, nil
	c.installed, c.stateErr = cloneBundleSelection(result.installed), result.stateErr
	c.applyErr, c.output = result.applyErr, result.output
	if c.applyErr == nil {
		c.selected = cloneBundleSelection(c.installed)
	}
	c.logOffset = 0
}

func (m *Model) communityApply() tea.Cmd {
	c := &m.community
	if c.busy || c.stateErr != nil {
		return nil
	}
	args := []string{"__community-bundles"}
	for _, bundle := range community.Bundles() {
		if c.selected[bundle.ID] {
			args = append(args, bundle.ID)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.busy, c.cancel = true, cancel
	c.applyErr, c.output = nil, ""
	home, capture := m.options.Home, m.options.Background
	return func() tea.Msg {
		defer cancel()
		var output string
		var applyErr error
		if capture != nil {
			output, applyErr = capture(ctx, args)
		} else {
			var executable string
			executable, applyErr = os.Executable()
			if applyErr == nil {
				cmd := exec.CommandContext(ctx, executable, args...)
				cmd.Env = append(os.Environ(), "HOME="+home)
				var log bytes.Buffer
				cmd.Stdout, cmd.Stderr = &log, &log
				applyErr = cmd.Run()
				output = log.String()
			}
		}
		installed, stateErr := community.Installed(home)
		return communityResult{installed, stateErr, applyErr, sessions.DisplayLog(output)}
	}
}

func (m *Model) communityToggle() {
	c := &m.community
	bundles := community.Bundles()
	if c.busy || c.stateErr != nil || c.cursor >= len(bundles) {
		return
	}
	id := bundles[c.cursor].ID
	if c.selected[id] {
		delete(c.selected, id)
	} else {
		c.selected[id] = true
	}
}

func (m *Model) communityBindings() []string {
	if m.community.showLog {
		return []string{"esc Back", "↑↓ Scroll", "pgup/pgdown Scroll"}
	}
	bindings := []string{"↑↓ Move", "space Toggle", "enter Confirm", "esc Back"}
	if m.community.output != "" {
		bindings = append(bindings, "l Log")
	}
	return bindings
}

func (m *Model) communityKey(msg tea.KeyMsg) tea.Cmd {
	c := &m.community
	key := msg.String()
	last := len(community.Bundles())
	if key == "ctrl+c" {
		if c.cancel != nil {
			c.cancel()
		}
		return m.quit()
	}
	if c.showLog {
		r := m.communityBounds()
		switch key {
		case "esc", "l":
			c.showLog = false
		case "up":
			m.communityScrollLog(r, -1)
		case "down":
			m.communityScrollLog(r, 1)
		case "pgup":
			m.communityScrollLog(r, -r.h+1)
		case "pgdown":
			m.communityScrollLog(r, r.h-1)
		case "home":
			c.logOffset = 0
		case "end":
			c.logOffset = m.communityMaxLogOffset(r)
		}
		return nil
	}
	switch key {
	case "esc":
		m.open("skills.sh")
	case "up", "shift+tab":
		c.cursor = max(0, c.cursor-1)
	case "down", "tab":
		c.cursor = min(last, c.cursor+1)
	case "home":
		c.cursor = 0
	case "end":
		c.cursor = last
	case "enter", " ":
		if c.cursor == last {
			return m.communityApply()
		}
		m.communityToggle()
	case "l":
		if c.output != "" {
			c.showLog = true
		}
	}
	return nil
}

func (m *Model) communityBounds() rect {
	width := min(76, max(20, m.width)-4)
	return rect{(max(20, m.width) - width) / 2, 3, width, max(1, m.footerTop()-4)}
}

func (m *Model) communityRows(r rect) (start, limit, pitch int) {
	pitch = 1
	if r.h >= 10 {
		pitch = 2
	}
	visible := max(1, (r.h-2)/pitch)
	last := len(community.Bundles())
	start = max(0, m.community.cursor-visible+1)
	return start, min(last+1, start+visible), pitch
}

func (m *Model) communityMouse(msg tea.MouseMsg) tea.Cmd {
	r := m.communityBounds()
	if !r.contains(msg.X, msg.Y) {
		return nil
	}
	c := &m.community
	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
		delta := 1
		if msg.Button == tea.MouseButtonWheelUp {
			delta = -1
		}
		if c.showLog {
			m.communityScrollLog(r, delta*3)
		} else {
			c.cursor = max(0, min(len(community.Bundles()), c.cursor+delta))
		}
		return nil
	}
	if c.showLog || msg.Button != tea.MouseButtonLeft || msg.Action != tea.MouseActionPress {
		return nil
	}
	start, limit, pitch := m.communityRows(r)
	index := start + (msg.Y-r.y-1)/pitch
	if msg.Y > r.y && index < limit {
		c.cursor = index
		return m.communityKey(tea.KeyMsg{Type: tea.KeyEnter})
	}
	return nil
}

func (m *Model) communityMaxLogOffset(r rect) int {
	return max(0, wrappedLines(m.community.output, r.w)-max(1, r.h-1))
}

func (m *Model) communityScrollLog(r rect, delta int) {
	c := &m.community
	c.logOffset = max(0, min(m.communityMaxLogOffset(r), c.logOffset+delta))
}

func (m *Model) communityView(f *frame, a appearance) {
	r := m.communityBounds()
	c := &m.community
	if c.showLog {
		f.section(rect{r.x, r.y, r.w, 1}, m.tr("Installation log"), a)
		f.richText(rect{r.x, r.y + 1, r.w, r.h - 1}, c.output, a, c.logOffset)
		return
	}
	f.section(rect{r.x, r.y, r.w, 1}, m.tr("Manage bundles"), a)
	start, limit, pitch := m.communityRows(r)
	bundles := community.Bundles()
	for i := start; i < limit; i++ {
		y := r.y + 1 + (i-start)*pitch
		style, focus := a.base, " "
		if i == c.cursor {
			style, focus = a.selected, "›"
		}
		if i == len(bundles) {
			f.fill(rect{r.x, y, r.w, 1}, style.Bold(true))
			label := focus + " " + m.tr("Confirm")
			f.put(r.x, y, style.Bold(true).Render(ansi.Truncate(label, r.w, "…")))
			break
		}
		bundle := bundles[i]
		check := "[ ]"
		if c.selected[bundle.ID] {
			check = "[x]"
		}
		state := m.communityBundleState(bundle.ID)
		stateWidth := ansi.StringWidth(state)
		stateGap := 0
		if state != "" && r.w-stateWidth >= 19 {
			stateGap = stateWidth + 2
		}
		f.fill(rect{r.x, y, r.w, 1}, style)
		f.put(r.x, y, style.Render(ansi.Truncate(focus+" "+check+" "+m.tr(bundle.Label), r.w-stateGap, "…")))
		if stateGap != 0 {
			f.put(r.x+r.w-stateWidth-1, y, style.Render(state))
		}
		if pitch == 2 {
			f.put(r.x+6, y+1, a.muted.Render(ansi.Truncate(bundle.Source, r.w-6, "…")))
		}
	}
	status := m.tr("Check bundles to install. Clear bundles to remove.")
	switch {
	case c.stateErr != nil:
		status = fmt.Sprintf("%s: %v", m.tr("Could not read bundle status"), c.stateErr)
	case c.busy:
		status = m.tr("Applying bundle changes…")
	case c.applyErr != nil:
		status = fmt.Sprintf("%s: %v", m.tr("Could not apply all changes"), c.applyErr)
	case c.output != "":
		status = m.tr("Bundle changes applied")
	case !maps.Equal(c.installed, c.selected):
		status = m.tr("Review your selection, then confirm.")
	}
	if c.output != "" && !c.busy {
		status = m.localizedBinding("l Log") + " · " + status
	}
	// On a compact screen, give the selected bundle a readable source or status.
	if r.h >= 5 && r.h < 10 && c.cursor < len(bundles) && !c.busy && c.output == "" && c.applyErr == nil && c.stateErr == nil {
		bundle := bundles[c.cursor]
		status = strings.TrimSpace(m.communityBundleState(bundle.ID) + " " + bundle.Source)
	}
	y := min(m.footerTop()-1, r.y+r.h-1)
	f.put(r.x, y, a.muted.Render(ansi.Truncate(status, r.w, "…")))
}

func (m *Model) communityBundleState(id string) string {
	c := &m.community
	if c.stateErr != nil {
		return ""
	}
	if c.installed[id] {
		if c.selected[id] {
			return m.tr("Installed")
		}
		return m.tr("Will remove")
	}
	if c.selected[id] {
		return m.tr("Will install")
	}
	return m.tr("Available")
}
