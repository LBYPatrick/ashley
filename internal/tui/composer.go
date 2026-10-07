package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/LBYPatrick/ashley/internal/agents"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

var permissionNames = []string{"Ask first", "Full access", "Auto edits", "Autonomous"}
var permissionHelp = []string{
	"Use the agent's standard approval prompts.",
	"Skip approval prompts and allow unrestricted access.",
	"Use the agent's automatic edit approval mode.",
	"Work without input, with unrestricted access.",
}

func (m *Model) composeBounds() rect {
	w := max(20, m.width) - 4
	return rect{(max(20, m.width) - w) / 2, 3, w, max(5, m.height-6)}
}
func (m *Model) composerInput() rect {
	r := m.composeBounds()
	return rect{r.x, r.y + 5, r.w, max(3, min(10, m.height-18))}
}
func (m *Model) sizeComposer() {
	styleEditor(&m.composer, m.appearance())
	r := m.composerInput()
	m.composer.SetWidth(max(10, r.w-4))
	m.composer.SetHeight(max(1, r.h-2))
}
func (m *Model) composeKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+c":
		return m.quit()
	case "esc":
		m.open("hub")
		return nil
	case "ctrl+n":
		return m.beginName()
	case "ctrl+s":
		m.open("vibe")
		return nil
	case "ctrl+d":
		m.runSkill = "raw"
		return nil
	case "ctrl+r":
		return m.launchDraft()
	case "tab", "shift+tab":
		d := 1
		if msg.String() == "shift+tab" {
			d = -1
		}
		m.composeFocus = (m.composeFocus + d + 4) % 4
		m.composer.Blur()
		if m.composeFocus == 0 {
			return m.composer.Focus()
		}
		return nil
	}
	if m.composeFocus == 0 {
		var cmd tea.Cmd
		m.composer, cmd = m.composer.Update(msg)
		return cmd
	}
	if msg.String() == "enter" || msg.String() == " " || msg.String() == "right" || msg.String() == "left" {
		switch m.composeFocus {
		case 1:
			m.cycleAgent()
		case 2:
			d := 1
			if msg.String() == "left" {
				d = -1
			}
			m.mode = (m.mode + d + len(modes)) % len(modes)
		case 3:
			return m.launchDraft()
		}
	}
	return nil
}
func (m *Model) launchDraft() tea.Cmd {
	if strings.TrimSpace(m.composer.Value()) == "" && m.runSkill == "raw" {
		m.status = "Describe a task, or choose a skill with Ctrl+S."
		m.composeFocus = 0
		return m.composer.Focus()
	}
	flags := []string{"--normal", "--dangerously-skip-permissions", "--auto", "--away-from-keyboard"}
	args := []string{"run", agentFlag(m.agent), flags[m.mode]}
	if m.runName != "" {
		args = append(args, "--name="+m.runName)
	}
	args = append(args, m.runSkill, "--", m.composer.Value())
	return m.execute(args...)
}
func (m *Model) composeView(f *frame, a appearance) {
	r, in := m.composeBounds(), m.composerInput()
	f.section(rect{r.x, r.y, r.w, 1}, "Start a new conversation", a)
	name := m.runName
	if name == "" {
		name = "Untitled"
	}
	f.put(r.x, r.y+2, a.muted.Render(ansi.Truncate("Name  "+name+"   ^N edit", r.w, "…")))
	cwd, _ := os.Getwd()
	f.put(r.x, r.y+1, a.muted.Render(ansi.Truncate(cwd, r.w, "…")))
	skill := "No skill · a direct conversation"
	if m.runSkill != "raw" {
		skill = "a-" + m.runSkill
	}
	f.put(r.x, r.y+3, a.title.Render(ansi.Truncate("Skill  "+skill+"   ^S choose · ^D clear", r.w, "…")))
	border := a.border
	if m.composeFocus == 0 {
		border = a.title
	}
	f.box(in, border)
	f.text(rect{in.x + 2, in.y + 1, in.w - 4, in.h - 2}, m.composer.View(), a.base, 0)
	y := in.y + in.h + 1
	for i, text := range []string{"Agent        " + agents.Get(m.agent).Label, "Permissions  " + permissionNames[m.mode]} {
		style := a.base
		marker := "  "
		if m.composeFocus == i+1 {
			style = a.selected
			marker = "› "
		}
		f.put(r.x, y+i, style.Render(ansi.Truncate(marker+text, r.w, "…")))
	}
	hint := permissionHelp[m.mode]
	if m.mode != 0 && (m.agent == "opencode" || m.agent == "kilo") {
		hint = "Automatic approvals; the agent's explicit deny rules still apply."
	}
	f.put(r.x+2, y+3, a.muted.Render(ansi.Truncate(hint, r.w-2, "…")))
	style := a.title
	if m.composeFocus == 3 {
		style = a.selected
	}
	f.put(r.x, y+5, style.Render(" Start conversation  ^R "))
	if r.w > 65 {
		f.put(r.x+30, y+5, a.muted.Render(fmt.Sprintf("%s · %s", agents.Get(m.agent).Label, permissionNames[m.mode])))
	}
}
