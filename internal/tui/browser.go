package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/LBYPatrick/ashley/internal/agents"
	"github.com/charmbracelet/x/ansi"
)

func skillRank(name, query string) int {
	if name == query || "a-"+name == query {
		return 0
	}
	if strings.HasPrefix(name, query) {
		return 1
	}
	if strings.Contains(name, query) {
		return 2
	}
	return 3
}

func (m *Model) rowHeight() int {
	if m.isActivity() {
		if m.height >= 20 {
			return min(3, m.layout().list.h)
		}
		if m.height >= 13 {
			return min(2, m.layout().list.h)
		}
	}
	return 1
}
func (m *Model) visibleRows() int { return max(1, m.layout().list.h/m.rowHeight()) }

func (m *Model) browserView(f *frame, a appearance) {
	l := m.layout()
	var titles, subtitles, directories []string
	heading, hint := "Choose a skill", "/ Search by name or purpose"
	switch m.screen {
	case "vibe":
		for _, name := range m.names {
			titles = append(titles, "a-"+name)
		}
	case "skills.sh":
		heading, hint = "Skill library", "Discover · maintain · create"
		for _, v := range skillsActions {
			titles = append(titles, v.label)
		}
	case "sessions":
		heading, hint = "Activity · Sessions", "t History   r Refresh   s Sort"
		for _, s := range m.sessionRows {
			state := "Finished"
			if alive, known := m.sessionAlive[s.ID]; !known {
				state = "Checking"
			} else if alive {
				state = "Running"
			}
			titles = append(titles, taskTitle(s.Skill, s.Question, s.Name))
			subtitles = append(subtitles, state+" · "+s.Skill)
			directories = append(directories, s.CWD)
		}
	case "history":
		heading = fmt.Sprintf("History (%d)", m.total)
		hint = fmt.Sprintf("Page %d/%d · n Next · p Previous", m.offset/50+1, max(1, (m.total+49)/50))
		for _, v := range m.historyRows {
			titles = append(titles, taskTitle(v.Skill, v.Question, v.Name))
			subtitles = append(subtitles, v.Skill+" · "+agents.Get(v.AgentType).Label+" · "+v.TimeDisplay())
			directories = append(directories, v.CWD)
		}
	}
	if !m.compactDetail {
		f.section(rect{l.left.x, l.left.y, l.left.w, 1}, heading, a)
		if m.isActivity() {
			f.fill(rect{l.left.x, l.left.y, l.left.w, 1}, a.base)
			for _, control := range m.activityTabControls() {
				style := a.muted
				if control.key == m.screen {
					style = a.selected
				}
				f.put(control.x, control.y, style.Render(" "+control.label+" "))
			}
		}
		if m.isActivity() {
			m.activitySearchView(f, a)
		} else if m.screen == "vibe" {
			f.input(l.input, m.filter.Value(), "Search skills…", m.focus == "filter", a, m.filter.Position())
		} else {
			f.put(l.left.x, l.left.y+1, a.muted.Render(ansi.Truncate(hint, l.left.w, "…")))
		}
		for _, control := range m.sessionActionControls() {
			f.put(control.x, control.y, a.panel.Render(" "+control.label+" "))
		}
		visible := m.visibleRows()
		start := max(0, m.cursor-visible+1)
		for i := start; i < min(len(titles), start+visible); i++ {
			style, marker := a.base, "  "
			if i == m.cursor {
				style, marker = a.selected, "› "
			}
			y := l.list.y + (i-start)*m.rowHeight()
			title, titleWidth, suffix := titles[i], l.list.w, ""
			if m.screen == "sessions" {
				indicator := "○ "
				if alive, known := m.sessionAlive[m.sessionRows[i].ID]; !known {
					indicator = "? "
				} else if alive {
					indicator = "● "
				}
				title = indicator + title
				if l.list.w >= 12 {
					suffix = " " + m.sessionRows[i].Elapsed(time.Now())
					titleWidth = max(4, titleWidth-ansi.StringWidth(suffix))
				}
			}
			if m.isActivity() && m.rowHeight() == 1 {
				directoryWidth := max(1, (l.list.w-ansi.StringWidth(suffix)-6)/2)
				suffix = " " + directoryLabel(directories[i], directoryWidth) + suffix
				titleWidth = max(1, l.list.w-ansi.StringWidth(suffix))
			}
			label := ansi.Truncate(marker+title, titleWidth, "…")
			f.put(l.list.x, y, style.Render(label+strings.Repeat(" ", max(0, titleWidth-ansi.StringWidth(label)))+suffix))
			if m.rowHeight() > 1 && i < len(subtitles) {
				text := subtitles[i]
				if m.rowHeight() == 2 {
					text = directoryLabel(directories[i], l.list.w-2)
				}
				f.put(l.list.x+2, y+1, a.muted.Render(ansi.Truncate(text, l.list.w-2, "…")))
				if m.rowHeight() == 3 {
					f.put(l.list.x+2, y+2, a.muted.Render(directoryLabel(directories[i], l.list.w-2)))
				}
			}
		}
		if len(titles) == 0 {
			text := "Nothing here yet.\nCtrl+2 starts a new conversation."
			if m.isActivity() && strings.TrimSpace(m.filter.Value()) != "" {
				text = "No matching results.\nCtrl+U clears the search."
			}
			f.text(rect{l.list.x, l.list.y, l.list.w, min(4, l.list.h)}, text, a.muted, 0)
		}
		if len(titles) > visible && m.height >= 12 {
			f.put(l.list.x, m.height-3, a.muted.Render(fmt.Sprintf("%d of %d", m.cursor+1, len(titles))))
		}
		if m.screen == "history" && m.height >= 12 {
			f.put(l.list.x, m.height-3, a.muted.Render(ansi.Truncate(fmt.Sprintf("%d total · ", m.total)+hint, l.list.w, "…")))
		}
	}
	if l.detail.w > 0 && !m.maximized {
		f.section(rect{l.detail.x, l.right.y, l.detail.w, 1}, "DETAILS", a)
		if !m.compactDetail {
			for y := 3; y < m.height-3; y++ {
				f.put(l.right.x-2, y, a.border.Render("│"))
			}
		}
		if m.screen == "sessions" && (len(m.sessionRows) > 0 || m.sessionLog != "") {
			detail, log, body := m.sessionPanels()
			f.richText(detail, m.detailText(), a, m.preview.YOffset)
			f.put(log.x, log.y, a.border.Render(strings.Repeat("─", log.w)))
			f.put(log.x, log.y+1, a.muted.Render("Recent output · L opens full log"))
			f.text(body, m.sessionLog, a.base, min(m.logOffset, m.maxLogOffset()))
		} else {
			f.richText(l.detail, m.detailText(), a, m.preview.YOffset)
		}
	}
	if m.screen == "skills.sh" && m.skillsHasInput() {
		f.input(l.input, m.question.Value(), m.skillsPlaceholder(), m.focus == "skills-input", a, m.question.Position())
	}
}
