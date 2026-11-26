package ui

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/shenri1/lazymanager/internal/history"
	"github.com/shenri1/lazymanager/internal/manager"
)

func TestCommandLine(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"sudo", "-n", "apt", "remove", "-y", "htop"}, "sudo -n apt remove -y htop"},
		{[]string{"sh", "-c", "apt list --upgradable 2>/dev/null"}, `sh -c 'apt list --upgradable 2>/dev/null'`},
		{[]string{"printf", "it's"}, `printf 'it'\''s'`},
	}
	for _, tc := range cases {
		if got := commandLine(exec.Command(tc.args[0], tc.args[1:]...)); got != tc.want {
			t.Errorf("commandLine(%v) = %q, want %q", tc.args, got, tc.want)
		}
	}

	// Control characters must not reach the screen or the audit file.
	got := commandLine(exec.Command("echo", "a\x1b[31mb\nc"))
	if strings.ContainsAny(got, "\x1b\n") {
		t.Errorf("commandLine kept control characters: %q", got)
	}
}

func TestConfirmShowsExactCommand(t *testing.T) {
	mgr := manager.Manager{
		Name:       "brew",
		Kind:       "system",
		Remove:     []string{"brew", "uninstall"},
		Upgrade:    []string{"brew", "upgrade"},
		UpgradeAll: []string{"brew", "upgrade"},
	}
	m := New("/tmp", []manager.Manager{mgr})
	m.width, m.height = 120, 30

	m.confirm = confirmRemove
	m.confirmAt = "htop"
	if bar := m.statusBar(); !strings.Contains(bar, "Remove htop? $ brew uninstall htop (y/n)") {
		t.Errorf("remove confirmation = %q", bar)
	}

	m.confirm = confirmUpgrade
	if bar := m.statusBar(); !strings.Contains(bar, "Upgrade htop? $ brew upgrade htop (y/n)") {
		t.Errorf("upgrade confirmation = %q", bar)
	}

	m.confirm = confirmUpgradeAll
	if bar := m.statusBar(); !strings.Contains(bar, "Upgrade all packages? $ brew upgrade (y/n)") {
		t.Errorf("upgrade-all confirmation = %q", bar)
	}

	m.confirm = confirmUpgradeEverything
	m.confirmTargets = []manager.Manager{mgr, {Name: "flatpak", Kind: "system", UpgradeAll: []string{"flatpak", "update", "-y"}}}
	m.updatesCache = map[string][]manager.Package{"brew": make([]manager.Package, 2), "flatpak": make([]manager.Package, 1)}
	bar := m.statusBar()
	if !strings.Contains(bar, "$ brew upgrade (+1 more)") {
		t.Errorf("upgrade-everything confirmation = %q", bar)
	}
}

func TestBeginRunShowsCommandHeader(t *testing.T) {
	m := New("/tmp", nil)
	m.width, m.height = 100, 30
	m.resize()
	m.beginRun("test", []step{{mgr: manager.Manager{Name: "brew"}, cmd: exec.Command("true")}})

	if got := m.details.View(); !strings.Contains(got, "$ true") {
		t.Errorf("details header missing command: %q", got)
	}
	if len(m.runLines) == 0 || !strings.Contains(ansi.Strip(m.runLines[0]), "$ true") {
		t.Errorf("runLines = %#v", m.runLines)
	}
}

func TestBeginRunShowsOneCommandPerStep(t *testing.T) {
	m := New("/tmp", nil)
	m.width, m.height = 100, 30
	m.resize()
	m.beginRun("test", []step{
		{mgr: manager.Manager{Name: "dnf"}, cmd: exec.Command("true")},
		{mgr: manager.Manager{Name: "flatpak"}, cmd: exec.Command("true")},
	})
	plain := ansi.Strip(strings.Join(m.runLines, "\n"))
	for _, want := range []string{"── dnf ──", "$ true", "── flatpak ──"} {
		if !strings.Contains(plain, want) {
			t.Errorf("runLines missing %q:\n%s", want, plain)
		}
	}
}

func TestFinishedCommandIsRecorded(t *testing.T) {
	old := appendHistory
	defer func() { appendHistory = old }()
	var saved history.Entry
	appendHistory = func(e history.Entry) error {
		saved = e
		return nil
	}

	m := New("/tmp", nil)
	m.running = true
	e := history.Entry{
		Time:     time.Date(2026, 11, 26, 18, 0, 0, 0, time.UTC),
		Manager:  "apt",
		Command:  "sudo -n apt remove -y htop",
		ExitCode: 0,
		Duration: 150 * time.Millisecond,
	}
	next, _ := m.Update(runMsg{command: &e})
	m = next.(model)
	if len(m.cmdLog) != 1 || m.cmdLog[0].Command != e.Command {
		t.Errorf("cmdLog = %#v", m.cmdLog)
	}
	if saved.Command != e.Command || saved.ExitCode != 0 {
		t.Errorf("saved = %#v", saved)
	}
}

func TestLogView(t *testing.T) {
	m := New("/tmp", nil)
	m.width, m.height = 100, 30
	m.resize()
	m.cmdLog = []history.Entry{{
		Time:     time.Date(2026, 11, 26, 18, 0, 0, 0, time.UTC),
		Manager:  "apt",
		Command:  "sudo -n apt remove -y htop",
		ExitCode: 0,
		Duration: 250 * time.Millisecond,
	}}
	m.showLog = true
	out := m.View()
	for _, want := range []string{"Command Log (1)", "EXIT", "sudo -n apt remove -y htop", "250ms", "18:00:00"} {
		if !strings.Contains(out, want) {
			t.Errorf("log view missing %q:\n%s", want, out)
		}
	}
	for i, line := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(line); w > m.width {
			t.Errorf("line %d width %d > %d", i, w, m.width)
		}
	}
}

func TestLogViewEmpty(t *testing.T) {
	m := New("/tmp", nil)
	m.width, m.height = 100, 30
	m.resize()
	m.showLog = true
	if out := m.View(); !strings.Contains(out, "no commands in this session yet") {
		t.Errorf("empty log = %q", out)
	}
}

func TestCommandLogKeys(t *testing.T) {
	m := New("/tmp", nil)
	m.width, m.height = 100, 30
	m.resize()
	for _, key := range []rune{'@', '5'} {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
		m = next.(model)
		if !m.showLog {
			t.Fatalf("%q did not open the command log", key)
		}
		next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
		m = next.(model)
		if m.showLog {
			t.Fatalf("%q did not close the command log", key)
		}
	}
}

func TestExitCodeNilIsSuccess(t *testing.T) {
	if got := exitCode(nil); got != 0 {
		t.Errorf("exitCode(nil) = %d, want 0", got)
	}
}

func TestFitSuffixKeepsPrompt(t *testing.T) {
	got := fitSuffix("a very long confirmation that does not fit", " (y/n)", 30)
	if !strings.HasSuffix(got, " (y/n)") {
		t.Errorf("fitSuffix = %q, want the prompt suffix", got)
	}
	if w := ansi.StringWidth(got); w > 30 {
		t.Errorf("width = %d, want <= 30", w)
	}
}

func TestAddCommandPersists(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := New("/tmp", nil)
	m.addCommand(history.Entry{
		Time:     time.Date(2026, 11, 26, 18, 0, 0, 0, time.UTC),
		Manager:  "apt",
		Command:  "sudo -n apt remove -y htop",
		ExitCode: 0,
		Duration: 100 * time.Millisecond,
	})
	data, err := os.ReadFile(history.Path())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "$ sudo -n apt remove -y htop") {
		t.Errorf("history file = %q", data)
	}
}
