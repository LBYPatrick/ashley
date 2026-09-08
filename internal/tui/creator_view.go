package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func (m *Model) sizeCreatorEditor() {
	r := m.readingRect()
	a := m.appearance()
	styleEditor(&m.editor, a)
	m.editor.SetWidth(r.w)
	m.editor.SetHeight(max(3, r.h-3))
	if w := m.wizard; w != nil {
		styleEditor(&w.input, a)
		width, height := r.w-4, 1
		if w.stage == 0 && w.field == 3 {
			height = 3
		}
		if w.stage == 2 {
			width = max(8, r.w-min(22, max(10, r.w/3))-4)
			height = max(1, r.h-7)
		}
		w.input.SetWidth(max(8, width))
		w.input.SetHeight(height)
		m.preview.Width = r.w
		m.preview.Height = max(1, r.h-5)
		if w.stage == 3 {
			offset := m.preview.YOffset
			m.preview.SetContent(ansi.Wrap(m.logContent, r.w, ""))
			m.preview.SetYOffset(offset)
		}
	}
}
func (m *Model) creatorView(f *frame, a appearance) {
	r := m.readingRect()
	w := m.wizard
	if w == nil {
		f.put(r.x, r.y, a.base.Bold(true).Render("Edit skill JSON"))
		f.text(rect{r.x, r.y + 2, r.w, r.h - 2}, m.editor.View(), a.base, 0)
		return
	}
	content := f
	if w.stage == 0 {
		content = newFrame(f.width, 33, a.base)
	}
	stages := []string{"Basics", "Files", "Workflow", "Preview"}
	content.put(r.x, 3, a.base.Bold(true).Render(fmt.Sprintf("Create skill · Step %d/4 — %s", w.stage+1, stages[w.stage])))
	x := r.x
	for i, label := range stages {
		style := a.muted
		if i == w.stage {
			style = a.title
		}
		text := fmt.Sprintf("%d %s", i+1, label)
		content.put(x, 5, style.Render(ansi.Truncate(text, max(0, r.x+r.w-x), "")))
		x += ansi.StringWidth(text) + 3
	}
	switch w.stage {
	case 0:
		labels := []string{"Name", "Description", "Extends (optional)", "Preamble"}
		placeholders := []string{"my-skill", "What this skill does", "Parent skill name, or leave blank", "Instructions for the coding agent"}
		for i, y := range []int{8, 13, 18, 23} {
			value := w.basics[i]
			if i == w.field {
				value = w.input.Value()
			}
			content.put(r.x, y-1, a.base.Bold(true).Render(labels[i]))
			height := 3
			if i == 3 {
				height = 5
			}
			box := rect{r.x, y, r.w, height}
			content.fill(box, a.panel)
			edge := a.border
			if i == w.field {
				edge = a.title
			}
			content.put(box.x, box.y+height-1, edge.Render(strings.Repeat("─", box.w)))
			if i == w.field {
				content.text(rect{box.x + 2, box.y + 1, box.w - 4, height - 2}, w.input.View(), a.panel, 0)
			} else {
				style := a.panel
				if value == "" {
					value = placeholders[i]
					style = a.muted.Background(a.panel.GetBackground())
				}
				content.text(rect{box.x + 2, box.y + 1, box.w - 4, height - 2}, value, style, 0)
			}
		}
		content.put(r.x, 30, a.selected.Render(" Next → "))
		content.put(r.x+13, 30, a.muted.Render("Ctrl+N Next · Ctrl+S Save"))
		for y := 1; y < f.height-1; y++ {
			source := y + m.screenScroll
			if source < content.height {
				f.put(0, y, content.row(source))
			}
		}
	case 1:
		f.put(r.x, 7, a.muted.Render(ansi.Truncate("Space selects · leave empty to use defaults or inherited files", r.w, "…")))
		visible := max(1, m.height-11)
		start := max(0, w.cursor-visible+1)
		for i := start; i < min(len(w.files), start+visible); i++ {
			mark := "○"
			if w.selected[w.files[i]] {
				mark = "✓"
			}
			style := a.base
			pointer := " "
			if i == w.cursor {
				style = a.selected
				pointer = "›"
			}
			label := ansi.Truncate(pointer+" "+mark+" "+w.files[i], r.w, "")
			f.put(r.x, 9+i-start, style.Render(label+strings.Repeat(" ", max(0, r.w-ansi.StringWidth(label)))))
		}
	case 2:
		f.put(r.x, 7, a.muted.Render(ansi.Truncate(fmt.Sprintf("Step %d/%d · Ctrl+A add · Ctrl+D remove · Ctrl+←/→ switch", min(w.step+1, len(w.steps)), len(w.steps)), r.w, "…")))
		left := min(22, max(10, r.w/3))
		f.put(r.x, 8, a.muted.Render(ansi.Truncate(w.fields()[w.field], r.w, "…")))
		for i, label := range w.fields() {
			label = strings.Split(label, " (")[0]
			pointer := "  "
			style := a.base
			if i == w.field {
				pointer = "› "
				style = a.selected
			}
			f.put(r.x, 9+i*2, style.Render(ansi.Truncate(pointer+label, left, "…")))
		}
		f.fill(rect{r.x + left + 2, 9, max(1, r.w-left-2), max(1, r.h-6)}, a.panel)
		f.text(rect{r.x + left + 3, 10, max(1, r.w-left-4), max(1, r.h-7)}, w.input.View(), a.panel, 0)
	case 3:
		f.text(rect{r.x, 8, r.w, max(1, r.h-5)}, m.preview.View(), a.base, 0)
	}
}
func (m *Model) creatorMouse(msg tea.MouseMsg) tea.Cmd {
	w := m.wizard
	if w == nil {
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		return cmd
	}
	if w.stage == 0 {
		if msg.Button == tea.MouseButtonWheelUp {
			m.screenScroll = max(0, m.screenScroll-3)
			return nil
		}
		if msg.Button == tea.MouseButtonWheelDown {
			m.screenScroll = min(max(0, 33-m.height), m.screenScroll+3)
			return nil
		}
		r := m.readingRect()
		if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress && msg.X >= r.x && msg.X < r.x+r.w && msg.Y > 0 && msg.Y < m.height-1 {
			y := msg.Y + m.screenScroll
			for i, top := range []int{8, 13, 18, 23} {
				height := 3
				if i == 3 {
					height = 5
				}
				if y >= top && y < top+height {
					w.collect()
					w.field = i
					w.loadField()
					m.sizeCreatorEditor()
					return nil
				}
			}
			if y == 30 {
				return m.wizardKey(tea.KeyMsg{Type: tea.KeyCtrlN})
			}
		}
		return nil
	}
	if w.stage == 1 {
		if msg.Button == tea.MouseButtonWheelUp {
			return m.wizardKey(tea.KeyMsg{Type: tea.KeyUp})
		}
		if msg.Button == tea.MouseButtonWheelDown {
			return m.wizardKey(tea.KeyMsg{Type: tea.KeyDown})
		}
		r := m.readingRect()
		visible := max(1, m.height-11)
		start := max(0, w.cursor-visible+1)
		index := start + msg.Y - 9
		if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress && msg.X >= r.x && msg.X < r.x+r.w && msg.Y >= 9 && index < min(len(w.files), start+visible) {
			w.cursor = index
			w.selected[w.files[index]] = !w.selected[w.files[index]]
		}
		return nil
	}
	if w.stage == 2 && msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
		r := m.readingRect()
		index := (msg.Y - 9) / 2
		if msg.Y >= 9 && msg.X >= r.x && msg.X < r.x+min(22, max(10, r.w/3)) && index < len(w.fields()) {
			w.collect()
			w.field = index
			w.loadField()
			m.sizeCreatorEditor()
			return nil
		}
	}
	if w.stage == 2 {
		var cmd tea.Cmd
		w.input, cmd = w.input.Update(msg)
		return cmd
	}
	return m.wizardMouse(msg)
}

func styleEditor(editor *textarea.Model, a appearance) {
	editor.Prompt = ""
	editor.EndOfBufferCharacter = ' '
	style := textarea.Style{Base: a.panel, Text: a.panel, CursorLine: a.panel, LineNumber: a.muted, CursorLineNumber: a.muted, EndOfBuffer: a.panel, Placeholder: a.muted.Background(a.panel.GetBackground()), Prompt: a.panel}
	editor.FocusedStyle = style
	editor.BlurredStyle = style
	editor.Cursor.Style = a.panel.Reverse(true)
	if editor.Focused() {
		editor.Focus()
	} else {
		editor.Blur()
	}
}
