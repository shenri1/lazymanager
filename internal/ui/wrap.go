package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// wrapDetails breaks every line of s to at most width terminal cells. Lines in
// the "Key: value" form keep continuation lines aligned with the value.
func wrapDetails(s string, width int) string {
	if width < 1 {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = wrapDetailLine(line, width)
	}
	return strings.Join(lines, "\n")
}

func wrapDetailLine(line string, width int) string {
	if line == "" || ansi.StringWidth(line) <= width {
		return line
	}
	i := strings.Index(line, ": ")
	if i <= 0 {
		return ansi.Wrap(line, width, "")
	}
	prefix := line[:i+2]
	prefixW := ansi.StringWidth(prefix)
	if prefixW >= width {
		return ansi.Wrap(line, width, "")
	}
	parts := strings.Split(ansi.Wrap(line[i+2:], width-prefixW, ""), "\n")
	indent := strings.Repeat(" ", prefixW)
	var b strings.Builder
	b.WriteString(prefix)
	b.WriteString(parts[0])
	for _, part := range parts[1:] {
		b.WriteByte('\n')
		b.WriteString(indent)
		b.WriteString(part)
	}
	return b.String()
}

// wrapOutput breaks command output lines to at most width terminal cells,
// preserving any ANSI styling.
func wrapOutput(s string, width int) string {
	if width < 1 {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if ansi.StringWidth(line) > width {
			lines[i] = ansi.Wrap(line, width, "")
		}
	}
	return strings.Join(lines, "\n")
}
