package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
)

// presentation styles only human-facing output. Prompt, JSON, and log streams
// bypass it so they remain usable in pipes and scripts.
type presentation struct {
	out   io.Writer
	color bool
	width int
}

func present(out io.Writer) presentation {
	p := presentation{out: out, width: 100}
	if f, ok := out.(*os.File); ok && term.IsTerminal(f.Fd()) {
		p.color = os.Getenv("NO_COLOR") == "" && os.Getenv("ASHLEY_NO_COLOR") == "" && os.Getenv("TERM") != "dumb"
		if w, _, err := term.GetSize(f.Fd()); err == nil {
			p.width = max(20, w)
		}
	}
	return p
}
func (p presentation) styled(text, code string) string {
	if p.color {
		return "\x1b[" + code + "m" + text + "\x1b[0m"
	}
	return text
}
func (p presentation) heading(title string) {
	fmt.Fprintf(p.out, "\n  %s\n\n", p.styled("Ashley / "+title, "1;36"))
}
func (p presentation) section(title string) { fmt.Fprintf(p.out, "\n  %s\n", p.styled(title, "1")) }
func (p presentation) line(text string) {
	text = ansi.Strip(text)
	for _, line := range strings.Split(ansi.Hardwrap(text, max(10, p.width-4), true), "\n") {
		fmt.Fprintln(p.out, "  "+line)
	}
}
func (p presentation) field(label, value string) {
	if value == "" {
		value = "—"
	}
	if home, err := os.UserHomeDir(); err == nil {
		value = strings.ReplaceAll(value, home+string(os.PathSeparator), "~/")
	}
	p.line(fmt.Sprintf("%-12s %s", label, value))
}
func (p presentation) success(text string) {
	fmt.Fprintf(p.out, "\n  %s %s\n", p.styled("✓", "32"), text)
}
func (p presentation) table(headers []string, rows [][]string) {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = ansi.StringWidth(h)
	}
	for _, row := range rows {
		for i, v := range row {
			widths[i] = max(widths[i], ansi.StringWidth(ansi.Strip(v)))
		}
	}
	total := 2 + 2*(len(widths)-1)
	for _, w := range widths {
		total += w
	}
	// Narrow terminals use labeled records so long questions and paths remain readable.
	if total > p.width && len(headers) == 2 && widths[0] < p.width/3 {
		descriptionWidth := max(10, p.width-widths[0]-6)
		p.line(fmt.Sprintf("%-*s  %s", widths[0], headers[0], headers[1]))
		for _, row := range rows {
			lines := strings.Split(ansi.Wrap(ansi.Strip(row[1]), descriptionWidth, ""), "\n")
			for index, line := range lines {
				label := ""
				if index == 0 {
					label = row[0]
				}
				p.line(fmt.Sprintf("%-*s  %s", widths[0], label, line))
			}
		}
		return
	}
	if total > p.width {
		for n, row := range rows {
			if n > 0 {
				fmt.Fprintln(p.out)
			}
			for i, v := range row {
				p.field(headers[i], v)
			}
		}
		return
	}
	render := func(row []string, bold bool) {
		var parts []string
		for i, v := range row {
			v = ansi.Strip(v)
			parts = append(parts, v+strings.Repeat(" ", widths[i]-ansi.StringWidth(v)))
		}
		line := strings.TrimRight(strings.Join(parts, "  "), " ")
		if bold {
			line = p.styled(line, "1")
		}
		fmt.Fprintln(p.out, "  "+line)
	}
	render(headers, true)
	for _, row := range rows {
		render(row, false)
	}
}

// Indent vendor output without buffering prompts that lack a trailing newline.
type indentedWriter struct {
	out   io.Writer
	start bool
}

func (w *indentedWriter) Write(data []byte) (int, error) {
	written := 0
	for len(data) > 0 {
		if w.start {
			if _, err := io.WriteString(w.out, "    "); err != nil {
				return written, err
			}
			w.start = false
		}
		end := bytes.IndexByte(data, '\n') + 1
		if end == 0 {
			end = len(data)
		}
		n, err := w.out.Write(data[:end])
		written += n
		if err != nil {
			return written, err
		}
		if n != end {
			return written, io.ErrShortWrite
		}
		w.start = data[end-1] == '\n'
		data = data[end:]
	}
	return written, nil
}

// WriteError renders command failures using the same terminal-aware layout.
func WriteError(out io.Writer, err error) {
	p := present(out)
	fmt.Fprintf(out, "\n  %s\n", p.styled("Error", "1;31"))
	p.line(err.Error())
	fmt.Fprintln(out)
}
