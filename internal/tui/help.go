package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

type shortcutHelp struct {
	origin        string
	context       []string
	search        textinput.Model
	offset        int
	width, height int
}

type shortcutGroup struct {
	title    string
	bindings []string
}

func (m *Model) openShortcutHelp() {
	if m.screen == "help" {
		m.help.offset = 0
		return
	}
	search := textinput.New()
	search.Prompt = ""
	search.CharLimit = 120
	m.help = shortcutHelp{origin: m.screen, context: m.fullFooterBindings(), search: search, width: m.width, height: m.height}
	m.screen = "help"
}

func (m *Model) shortcutGroups() []shortcutGroup {
	groups := []shortcutGroup{
		{"Navigation", []string{"^1 Home", "^2 New run", "^3 Activity", "^4 Library", "^5 Settings", "^P Commands", "? All keys", "esc Back", "^C Quit"}},
		{"New run", []string{"^R Start conversation", "^N Name", "^S Choose a skill", "^D Clear skill", "tab Options", "enter Select or add a line"}},
		{"Activity", []string{"enter Attach/Resume", "↑↓ Move", "/ Search", "^F Scope", "^U Clear", "N Rename", "l Log", "c Copy ID", "r Refresh", "s Sort", "i Metadata", "K Stop", "X Stop all", "D Clean finished", "d Delete", "n/p Pages", "v Details"}},
		{"Skill browser", []string{"/ Search", "enter Use skill", "pgup/pgdown Preview", "p Copy prompt", "i Metadata", "v Details"}},
		{"Library", []string{"↑↓ Move", "enter Open / Submit", "v Details", "esc Back"}},
		{"Creator", []string{"^N Next", "^S Save", "^E JSON", "^R Preview", "^A Add step", "^D Remove step"}},
		{"Settings", []string{"↑↓←→ Move", "tab/shift+tab Move", "enter Select", "esc Done"}},
		{"Community bundle", []string{"↑↓ Move", "space Toggle", "enter Confirm", "l Log", "esc Back"}},
	}
	if len(m.help.context) > 0 {
		groups = append([]shortcutGroup{{"Current screen", m.help.context}}, groups...)
	}
	return groups
}

func shortcutKeyLabel(key string) string {
	if strings.HasPrefix(key, "^") {
		return "Ctrl+" + strings.ToUpper(key[1:])
	}
	switch key {
	case "enter":
		return "Enter"
	case "esc":
		return "Esc"
	case "tab":
		return "Tab"
	case "tab/shift+tab":
		return "Tab / Shift+Tab"
	case "pgup/pgdown":
		return "PgUp / PgDn"
	case "pgdn":
		return "PgDn"
	case "home/end":
		return "Home / End"
	}
	return key
}

func (m *Model) helpLayout() (search, body rect) {
	w := min(108, max(20, m.width)-4)
	x := (max(20, m.width) - w) / 2
	return rect{x, 3, w - 7, 1}, rect{x, 5, w, max(1, m.footerTop()-5)}
}

type shortcutCell struct {
	action, key string
	keyWidth    int
}

type shortcutRow struct {
	left, right shortcutCell
	heading     string
}

func shortcutColumnWidth(width int) int {
	if width >= 86 {
		return (width - 4) / 2
	}
	return width
}

func (m *Model) shortcutCells(binding string, width int) []shortcutCell {
	key, label, ok := strings.Cut(binding, " ")
	if !ok {
		return nil
	}
	key = shortcutKeyLabel(key)
	keyWidth := min(ansi.StringWidth(key), width/2-1)
	keys := strings.Split(ansi.Hardwrap(key, keyWidth, false), "\n")
	labelWidth := width - keyWidth - 3
	actions := strings.Split(ansi.Hardwrap(ansi.Wrap(m.tr(label), labelWidth, ""), labelWidth, false), "\n")
	cells := make([]shortcutCell, max(len(keys), len(actions)))
	for i := range cells {
		if i < len(keys) {
			cells[i].key = keys[i]
			cells[i].keyWidth = keyWidth
		}
		if i < len(actions) {
			cells[i].action = actions[i]
		}
	}
	return cells
}

func (m *Model) helpRows(width int) []shortcutRow {
	var rows []shortcutRow
	query := strings.Fields(strings.ToLower(m.help.search.Value()))
	for _, group := range m.shortcutGroups() {
		var matches []string
		for _, binding := range group.bindings {
			key, label, ok := strings.Cut(binding, " ")
			if !ok {
				continue
			}
			haystack := strings.ToLower(strings.Join([]string{
				group.title, m.tr(group.title), key, shortcutKeyLabel(key), label, m.tr(label),
			}, " "))
			match := true
			for _, word := range query {
				if !strings.Contains(haystack, word) {
					match = false
					break
				}
			}
			if match {
				matches = append(matches, binding)
			}
		}
		if len(matches) == 0 {
			continue
		}
		if len(rows) > 0 {
			rows = append(rows, shortcutRow{})
		}
		rows = append(rows, shortcutRow{heading: m.tr(group.title)})
		columns := shortcutColumnWidth(width)
		leftCount := len(matches)
		if width >= 86 {
			leftCount = (len(matches) + 1) / 2
		}
		for i, binding := range matches[:leftCount] {
			left := m.shortcutCells(binding, columns)
			var right []shortcutCell
			if i+leftCount < len(matches) {
				right = m.shortcutCells(matches[i+leftCount], columns)
			}
			for j := range max(len(left), len(right)) {
				row := shortcutRow{}
				if j < len(left) {
					row.left = left[j]
				}
				if j < len(right) {
					row.right = right[j]
				}
				rows = append(rows, row)
			}
		}
	}
	return rows
}

func (m *Model) helpMaxOffset() int {
	_, body := m.helpLayout()
	return max(0, len(m.helpRows(body.w))-body.h)
}

func (m *Model) helpKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.screen = m.help.origin
		if m.screen == "" {
			m.screen = "hub"
		}
		if m.width != m.help.width || m.height != m.help.height {
			switch m.screen {
			case "log", "create-preview":
				m.sizeLogPreview()
			case "create":
				m.sizeCreatorEditor()
			case "hub", "vibe", "skills.sh", "activity", "sessions", "history":
				offset := m.preview.YOffset
				m.setPreviewContent()
				m.preview.SetYOffset(offset)
			case "settings":
				m.keepSettingVisible()
			case "about":
				m.scrollAbout(0)
			}
		}
		if m.screen == "activity" || m.screen == "sessions" {
			m.sessionStatusesDirty = true
			m.queueSessionDetails()
		}
	case "ctrl+c":
		return m.quit()
	case "enter":
		m.help.search.Blur()
	case "ctrl+u":
		m.help.search.SetValue("")
		m.help.offset = 0
	case "up", "down", "pgup", "pgdown":
		delta := 1
		if strings.HasPrefix(msg.String(), "pg") {
			_, body := m.helpLayout()
			delta = body.h
		}
		if msg.String() == "up" || msg.String() == "pgup" {
			delta = -delta
		}
		m.help.offset = max(0, min(m.helpMaxOffset(), m.help.offset+delta))
	default:
		if !m.help.search.Focused() {
			switch msg.String() {
			case "?", "home":
				m.help.offset = 0
			case "end":
				m.help.offset = m.helpMaxOffset()
			case "/":
				return m.help.search.Focus()
			}
		} else {
			before := m.help.search.Value()
			var cmd tea.Cmd
			m.help.search, cmd = m.help.search.Update(msg)
			if before != m.help.search.Value() {
				m.help.offset = 0
			}
			return cmd
		}
	}
	return nil
}

func (m *Model) helpMouse(msg tea.MouseMsg) tea.Cmd {
	search, body := m.helpLayout()
	if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
		if search.contains(msg.X, msg.Y) {
			return m.help.search.Focus()
		}
		if (rect{body.x + body.w - 5, search.y, 5, 1}).contains(msg.X, msg.Y) {
			return m.helpKey(tea.KeyMsg{Type: tea.KeyEsc})
		}
	}
	if body.contains(msg.X, msg.Y) {
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.help.offset = max(0, m.help.offset-3)
		case tea.MouseButtonWheelDown:
			m.help.offset = min(m.helpMaxOffset(), m.help.offset+3)
		}
	}
	return nil
}

func (m *Model) helpView(f *frame, a appearance) {
	search, body := m.helpLayout()
	placeholder := m.tr("Search shortcuts…")
	if !m.help.search.Focused() {
		placeholder = "/ " + placeholder
	}
	f.input(search, m.help.search.Value(), placeholder, m.help.search.Focused(), a, m.help.search.Position())
	f.put(body.x+body.w-5, search.y, a.panel.Bold(true).Render(" Esc "))
	rows := m.helpRows(body.w)
	if len(rows) == 0 {
		f.text(body, m.tr("No matching shortcuts"), a.muted, 0)
		return
	}
	offset := min(m.help.offset, max(0, len(rows)-body.h))
	columnWidth := shortcutColumnWidth(body.w)
	for i, row := range rows[offset:min(len(rows), offset+body.h)] {
		y := body.y + i
		if row.heading != "" {
			f.put(body.x, y, a.title.Render(ansi.Truncate(row.heading, body.w, "…")))
			continue
		}
		f.shortcutRow(body.x, y, columnWidth, row.left, a)
		f.shortcutRow(body.x+columnWidth+4, y, columnWidth, row.right, a)
	}
	f.scrollbar(rect{body.x + body.w, body.y, 1, body.h}, len(rows), offset, a)
}

func (f *frame) shortcutRow(x, y, width int, cell shortcutCell, a appearance) {
	if cell.key != "" {
		badge := " " + cell.key + strings.Repeat(" ", cell.keyWidth-ansi.StringWidth(cell.key)+1)
		f.put(x+width-cell.keyWidth-2, y, a.panel.Bold(true).Render(badge))
	}
	f.put(x, y, a.base.Render(cell.action))
}
