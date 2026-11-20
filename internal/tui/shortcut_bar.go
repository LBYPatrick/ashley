package tui

import (
	"strings"

	"github.com/LBYPatrick/ashley/internal/community"
	"github.com/LBYPatrick/ashley/internal/config"
	"github.com/charmbracelet/x/ansi"
)

func (m *Model) fullFooterBindings() []string {
	switch {
	case m.confirm != nil:
		bindings := []string{"tab/shift+tab Choose", "←→ Choose", "enter Accept", "esc/n Cancel", "^C Quit"}
		if m.maxConfirmationScroll() > 0 {
			bindings = append([]string{"↑↓/jk Read", "pgup/pgdown/space Page"}, bindings...)
		}
		return bindings
	case m.rename != nil:
		return []string{"enter Save", "esc Cancel", "^C Quit"}
	case m.paletteOpen:
		return []string{"↑↓ Move", "enter Select", "esc/^P Close", "^C Quit"}
	}

	bindings := m.footerBindings()
	var extra []string
	switch {
	case m.focus == "filter":
		if !m.isActivity() {
			bindings = []string{"enter Results", "tab/esc Done", "^P Commands"}
		}
	case m.focus == "skills-input":
		bindings = []string{"enter Submit", "tab/esc Done", "^P Commands"}
	default:
		switch m.screen {
		case "hub":
			extra = []string{"j/k Move", "esc Quit", "r Refresh"}
		case "activity", "sessions", "history":
			extra = []string{"j/k Move"}
			if m.width < 76 {
				extra = append(extra, "→ Details")
			}
		case "vibe":
			bindings = []string{"enter/tab Use skill", "↑↓/jk Move", "/ Search", "pgup/pgdown Preview",
				"p Copy prompt", "i Metadata", "r Refresh", "esc Back", "^P Commands", "? All keys"}
			if m.width < 76 {
				extra = []string{"v/→ Details"}
			}
		case "skills.sh":
			extra = []string{"j/k Move", "r Refresh"}
			if m.width < 76 {
				extra = append(extra, "→ Details")
			}
		case "compose":
			bindings = []string{"^R Start", "^N Name", "^S Skill", "^D Clear skill", "tab/shift+tab Options", "esc Home", "^P Commands"}
			switch m.composeFocus {
			case 0:
				extra = []string{"enter New line"}
			case 3:
				extra = []string{"enter/space/←→ Start"}
			default:
				extra = []string{"enter/space/←→ Change"}
			}
			if m.composeFocus != 0 {
				extra = append(extra, "? All keys")
			}
		case "settings":
			bindings = []string{"↑↓←→ Move", "tab/shift+tab Move", "enter/space Select", "esc Done", "^P Commands", "? All keys"}
		case "stats":
			extra = []string{"pgup/pgdown Scroll"}
		case "about":
			extra = []string{"j/k Scroll", "? All keys"}
		case "sync":
			extra = []string{"↑↓ Scroll", "pgup/pgdown Page", "home/end Ends", "? All keys"}
		case "community":
			bindings = []string{"↑↓ Move", "tab/shift+tab Move", "home/end Ends", "esc Back", "^P Commands", "? All keys"}
			switch {
			case m.community.showLog:
				bindings = []string{"↑↓ Scroll", "pgup/pgdown Page", "home/end Ends", "esc/l Back", "^P Commands", "? All keys"}
			case !m.community.busy && m.community.stateErr == nil:
				action := "enter/space Toggle"
				if m.community.cursor == len(community.Bundles()) {
					action = "enter/space Confirm"
				}
				extra = append(extra, action)
			}
			if !m.community.showLog && m.community.output != "" {
				extra = append(extra, "l Log")
			}
		case "help":
			bindings = []string{"esc Back", "↑↓ Scroll", "pgup/pgdown Page", "^U Clear", "^P Commands"}
			if m.help.search.Focused() {
				extra = []string{"enter Results"}
			} else {
				extra = []string{"/ Search", "home/end Ends", "? Top"}
			}
		case "create":
			bindings = m.creatorFooterBindings()
		case "create-preview":
			bindings = []string{"esc Back to editor", "pgup/pgdown Scroll", "^P Commands", "? All keys"}
		}
	}
	return append(append(bindings, extra...), "^C Quit")
}

func (m *Model) creatorFooterBindings() []string {
	if m.wizard == nil {
		return []string{"^S Save", "^R Preview", "esc Back", "^P Commands"}
	}
	bindings := []string{"^S Save", "^E JSON", "esc/^B Back", "^P Commands"}
	if m.wizard.stage < 3 {
		bindings = append(bindings, "^N Next")
	}
	switch m.wizard.stage {
	case 0:
		bindings = append(bindings, "tab/shift+tab Field")
	case 1:
		bindings = append(bindings, "↑↓/jk Move", "enter/space Toggle")
	case 2:
		bindings = append(bindings, "tab/shift+tab Field", "^A Add step", "^D Remove step", "^←/^→ Switch step")
	case 3:
		bindings = append(bindings, "↑↓/jk Scroll", "←→/hl Scroll", "pgup/pgdown Page", "space/f/b Page", "u/d/^U/^D Half page")
	}
	return bindings
}

func (m *Model) wrapFooter(bindings []string, width int) []string {
	var rows []string
	var row string
	for _, binding := range bindings {
		hint := m.localizedBinding(binding)
		if row != "" && ansi.StringWidth(row)+2+ansi.StringWidth(hint) <= width {
			row += "  " + hint
			continue
		}
		if row != "" {
			rows = append(rows, row)
		}
		lines := strings.Split(ansi.Hardwrap(ansi.Wrap(hint, width, ""), width, false), "\n")
		rows = append(rows, lines[:len(lines)-1]...)
		row = lines[len(lines)-1]
	}
	if row != "" {
		rows = append(rows, row)
	}
	return rows
}

const footerPager = "  F6 ›"

func (m *Model) fullFooterLayout() (rows []string, visible int) {
	width := max(16, m.width-4)
	// Keep the page usable on short terminals. Dialogs share a single footer row.
	limit := max(1, m.height-15)
	if m.confirm != nil || m.rename != nil || m.paletteOpen {
		limit = 1
	}
	bindings := m.fullFooterBindings()
	rows = m.wrapFooter(bindings, width)
	if len(rows) <= limit {
		return rows, len(rows)
	}
	return m.wrapFooter(bindings, width-ansi.StringWidth(footerPager)), limit
}

func (m *Model) fullFooterRows() []string {
	rows, visible := m.fullFooterLayout()
	if len(rows) <= visible {
		return rows
	}
	start := 0
	if m.footerContext == strings.Join(rows, "\n") {
		start = min(m.footerPage, (len(rows)-1)/visible) * visible
	}
	page := make([]string, visible)
	copy(page, rows[start:min(start+visible, len(rows))])
	page[visible-1] += footerPager
	return page
}

func (m *Model) nextFooterPage() {
	if m.interfacePrefs.ShortcutMode() != config.ShortcutBarFull {
		return
	}
	rows, visible := m.fullFooterLayout()
	if len(rows) <= visible {
		return
	}
	context := strings.Join(rows, "\n")
	if context != m.footerContext {
		m.footerContext, m.footerPage = context, 0
	}
	m.footerPage = (m.footerPage + 1) % ((len(rows) + visible - 1) / visible)
}
