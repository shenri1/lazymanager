package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/shenri1/lazymanager/internal/manager"
)

func checkWidth(t *testing.T, got string, width int) []string {
	t.Helper()
	lines := strings.Split(got, "\n")
	for i, line := range lines {
		if w := ansi.StringWidth(line); w > width {
			t.Errorf("line %d is %d cells wide, want <= %d: %q", i, w, width, ansi.Strip(line))
		}
	}
	return lines
}

func TestWrapDetailsSimpleText(t *testing.T) {
	got := wrapDetails("alpha beta gamma delta", 10)
	checkWidth(t, got, 10)
	if want := "alpha beta\ngamma\ndelta"; got != want {
		t.Errorf("wrapDetails() = %q, want %q", got, want)
	}
}

func TestWrapDetailsKeyValueIndent(t *testing.T) {
	in := "Summary: A super-fast templating language that borrows the best ideas from the existing templating languages."
	got := wrapDetails(in, 40)
	lines := checkWidth(t, got, 40)
	want := strings.Split("Summary: A super-fast templating\n"+
		"         language that borrows the best\n"+
		"         ideas from the existing\n"+
		"         templating languages.", "\n")
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d:\n%s", len(lines), len(want), got)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, lines[i], want[i])
		}
	}
}

func TestWrapDetailsLongURL(t *testing.T) {
	got := wrapDetails("Homepage: https://example.com/very/long/path", 20)
	lines := checkWidth(t, got, 20)
	if len(lines) < 2 {
		t.Fatalf("long URL was not wrapped: %q", got)
	}
	if !strings.HasPrefix(lines[0], "Homepage: ") {
		t.Errorf("first line = %q, want Homepage prefix", lines[0])
	}
	indent := strings.Repeat(" ", len("Homepage: "))
	for i := 1; i < len(lines); i++ {
		if !strings.HasPrefix(lines[i], indent) {
			t.Errorf("line %d = %q, want indent %q", i, lines[i], indent)
		}
	}
	var b strings.Builder
	for i, line := range lines {
		if i == 0 {
			line = strings.TrimPrefix(line, "Homepage: ")
		} else {
			line = strings.TrimPrefix(line, indent)
		}
		b.WriteString(line)
	}
	if want := "https://example.com/very/long/path"; b.String() != want {
		t.Errorf("URL content = %q, want %q", b.String(), want)
	}
}

func TestWrapDetailsWideCharacters(t *testing.T) {
	got := wrapDetails("Summary: 这是一个非常长的中文描述🚀🚀🚀🚀", 12)
	checkWidth(t, got, 12)
	if !strings.HasPrefix(got, "Summary: ") {
		t.Errorf("missing key prefix: %q", got)
	}
}

func TestWindowResizeRewrapsDetails(t *testing.T) {
	m := New("/tmp", []manager.Manager{{Name: "apt", Kind: "system"}})
	m.width, m.height = 120, 30
	m.resize()
	m.setDetails("Details", "Summary: A super-fast templating language that borrows the best ideas from the existing templating languages.")
	wide := m.details.TotalLineCount()

	m.width = 60
	m.resize()
	narrow := m.details.TotalLineCount()
	if narrow <= wide {
		t.Errorf("resize did not rewrap: %d lines at width %d, %d at width %d", narrow, m.details.Width, wide, 120)
	}
}

func TestRenderRunWrapsAndFollows(t *testing.T) {
	m := New("/tmp", []manager.Manager{{Name: "apt", Kind: "system"}})
	m.width, m.height = 80, 20
	m.resize()
	m.running = true
	m.runLines = []string{"$ " + strings.Repeat("x", 2000)}
	m.runFollow = true
	m.renderRun()

	if m.details.TotalLineCount() < 2 {
		t.Fatalf("run output was not wrapped: %d lines", m.details.TotalLineCount())
	}
	if m.details.YOffset == 0 {
		t.Error("run output did not follow the end")
	}
	for _, line := range strings.Split(m.details.View(), "\n") {
		if w := ansi.StringWidth(line); w > m.details.Width {
			t.Errorf("run line is %d cells wide, want <= %d", w, m.details.Width)
		}
	}

	wide := m.details.TotalLineCount()
	m.width = 60
	m.resize()
	if m.details.TotalLineCount() <= wide {
		t.Errorf("resize did not rewrap run output: %d lines, was %d", m.details.TotalLineCount(), wide)
	}
}
