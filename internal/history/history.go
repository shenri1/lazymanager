// Package history persists the commands run by the TUI for auditing.
package history

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Entry is one command execution.
type Entry struct {
	Time     time.Time
	Manager  string
	Command  string
	ExitCode int
	Duration time.Duration
}

// Path returns $XDG_STATE_HOME/lazymanager/history.log, or
// ~/.local/state/lazymanager/history.log when XDG_STATE_HOME is unset.
func Path() string {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "lazymanager", "history.log")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "state", "lazymanager", "history.log")
}

// Append adds e to the history file, creating the directory and file when
// needed. The file is private to the user (0600).
func Append(e Entry) error {
	path := Path()
	if path == "" {
		return os.ErrNotExist
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintln(f, e.Line())
	return err
}

// Line renders e as one log line.
func (e Entry) Line() string {
	t := e.Time
	if t.IsZero() {
		t = time.Now()
	}
	return fmt.Sprintf("%s manager=%s exit=%d duration=%s $ %s",
		t.Format(time.RFC3339), e.Manager, e.ExitCode, e.Duration.Round(time.Millisecond), oneLine(e.Command))
}

func oneLine(s string) string {
	return strings.NewReplacer("\n", " ", "\r", " ", "\t", " ").Replace(s)
}
