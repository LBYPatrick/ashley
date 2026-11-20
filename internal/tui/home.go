package tui

import (
	"fmt"
	"strings"

	"github.com/LBYPatrick/ashley/internal/agents"
	"github.com/charmbracelet/x/ansi"
)

var homeTitles = []string{"New run", "Activity", "Sync skills", "Create a skill", "Usage", "Settings", "Skill library", "About Ashley"}
var homeDescriptions = []string{"Describe a task. Add a skill if useful.", "Open running and finished sessions.", "Install your skills for detected agents.", "Turn a workflow into a reusable skill.", "Review your skills and agent usage.", "Choose your default agent and appearance.", "Discover and manage community skills.", "Read the version, project, and license information."}

func (m *Model) recentTop() int {
	return 6
}

func (m *Model) homeBounds() (rect, rect) {
	w := max(20, m.width) - 4
	x := (max(20, m.width) - w) / 2
	left := rect{x, 8, w, max(1, m.contentHeight()-11)}
	right := rect{}
	if m.width >= 90 {
		left.w = (w - 6) / 2
		right = rect{x + left.w + 6, 8, w - left.w - 6, max(1, m.contentHeight()-11)}
	}
	return left, right
}

func (m *Model) homeControls() []settingControl {
	left, right := m.homeBounds()
	var out []settingControl
	gap := 2
	if m.contentHeight() >= 28 {
		gap = 3
	}
	for i := range hub {
		y := left.y + i*gap
		if i >= 2 {
			y = left.y + 2*gap + 2 + i - 2
		}
		out = append(out, settingControl{rect: rect{left.x, y, left.w, 1}, row: i, label: m.tr(homeTitles[i])})
	}
	for i, v := range m.recent {
		r := rect{right.x, m.recentTop() + 2 + i*3, right.w, 2}
		if right.w == 0 {
			r = rect{left.x, left.y + 2*gap + 10 + i*2, left.w, 1}
		}
		out = append(out, settingControl{rect: r, row: len(hub) + i, label: taskTitle(v.Skill, v.Question, v.Name)})
	}
	if m.cursor < len(out) && (right.w == 0 || m.cursor < len(hub)) {
		offset := max(0, out[m.cursor].y-(m.contentHeight()-4))
		for i := range out {
			if right.w == 0 || out[i].row < len(hub) {
				out[i].y -= offset
			}
		}
	}
	return out
}

func taskTitle(skill, question string, names ...string) string {
	if len(names) > 0 && strings.TrimSpace(names[0]) != "" {
		return strings.Join(strings.Fields(names[0]), " ")
	}
	if strings.TrimSpace(question) != "" {
		return strings.Join(strings.Fields(question), " ")
	}
	return skill
}

func (m *Model) homeView(f *frame, a appearance) {
	left, right := m.homeBounds()
	f.put(left.x, 3, a.base.Bold(true).Render(ansi.Truncate(m.tr("What would you like to work on?"), f.width-left.x-2, "…")))
	subtitle := "Start something new, or pick up where you left off."
	if m.firstRun {
		subtitle = "Welcome to Ashley. Choose New run to get started."
	}
	f.put(left.x, 4, a.muted.Render(ansi.Truncate(m.tr(subtitle), f.width-left.x-2, "…")))
	controls := m.homeControls()
	f.section(rect{left.x, 6, left.w, 1}, m.tr("01  WORKSPACE"), a)
	for _, c := range controls {
		if c.y < 6 || c.y >= m.contentHeight()-2 {
			continue
		}
		style, marker := a.base, "  "
		if c.row == m.cursor {
			style, marker = a.selected, "› "
		}
		label := ansi.Truncate(marker+c.label, c.w, "…")
		f.put(c.x, c.y, style.Render(label+strings.Repeat(" ", max(0, c.w-ansi.StringWidth(label)))))
		if c.row < 2 && m.contentHeight() >= 28 {
			f.put(c.x+2, c.y+1, a.muted.Render(ansi.Truncate(m.tr(homeDescriptions[c.row]), c.w-2, "…")))
		}
		if c.row >= len(hub) && right.w > 0 {
			v := m.recent[c.row-len(hub)]
			f.put(c.x+2, c.y+1, a.muted.Render(ansi.Truncate(v.Skill+" · "+agents.Get(v.AgentType).Label, c.w-2, "…")))
		}
	}
	if len(controls) > 2 {
		f.section(rect{left.x, controls[2].y - 1, left.w, 1}, m.tr("03  YOUR TOOLS"), a)
	}
	if right.w == 0 {
		return
	}
	f.section(rect{right.x, m.recentTop(), right.w, 1}, m.tr("02  RECENT CONVERSATIONS"), a)
	if len(m.recent) == 0 {
		f.text(rect{right.x, m.recentTop() + 2, right.w, 4}, m.tr("Start a run to see it here or in Activity."), a.muted, 0)
	}
	if m.contentHeight() >= 28 {
		f.put(right.x, m.contentHeight()-4, a.muted.Render(ansi.Truncate(fmt.Sprintf("%s · Ctrl+5 to change defaults", agents.Get(m.agent).Label), right.w, "…")))
	}
}
