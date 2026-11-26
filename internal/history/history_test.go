package history

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPathHonorsXDGStateHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	if got, want := Path(), filepath.Join(dir, "lazymanager", "history.log"); got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func TestAppendWritesOneLine(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	e := Entry{
		Time:     time.Date(2026, 11, 26, 18, 30, 0, 0, time.UTC),
		Manager:  "apt",
		Command:  "sudo -n apt remove -y htop",
		ExitCode: 0,
		Duration: 250 * time.Millisecond,
	}
	if err := Append(e); err != nil {
		t.Fatal(err)
	}
	if err := Append(e); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(Path())
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2:\n%s", len(lines), data)
	}
	for _, want := range []string{"2026-11-26T18:30:00Z", "manager=apt", "exit=0", "duration=250ms", "$ sudo -n apt remove -y htop"} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("line missing %q: %s", want, lines[0])
		}
	}
	info, err := os.Stat(Path())
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("mode = %o, want 600", mode)
	}
}

func TestLineKeepsOneLine(t *testing.T) {
	e := Entry{Manager: "apt", Command: "a\nb\tc\rd"}
	line := e.Line()
	if strings.ContainsAny(line, "\n\t\r") {
		t.Errorf("line contains control whitespace: %q", line)
	}
}
