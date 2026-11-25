package ui

import (
	"fmt"
	"reflect"
	"testing"

	"lazymanager/internal/manager"
)

func TestStripANSI(t *testing.T) {
	in := "\x1b[31mred\x1b[0m and \x1b[1;32mgreen\x1b[0m\x1b]0;title\x07"
	want := "red and green"
	if got := stripANSI(in); got != want {
		t.Errorf("stripANSI(%q) = %q, want %q", in, got, want)
	}
}

func TestAppendOutput(t *testing.T) {
	lines := appendOutput(nil, "one", false)
	lines = appendOutput(lines, "two", false)
	lines = appendOutput(lines, "progress", true)

	want := []string{"one", "progress"}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("append/overwrite = %#v, want %#v", lines, want)
	}

	lines = appendOutput(lines, "\x1b[32mgreen\x1b[0m", false)
	want = []string{"one", "progress", "green"}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("ANSI stripped output = %#v, want %#v", lines, want)
	}

	lines = appendOutput(lines, "last", true)
	want = []string{"one", "progress", "last"}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("overwrite last line = %#v, want %#v", lines, want)
	}
}

func TestAppendOutputCap(t *testing.T) {
	var lines []string
	for i := 0; i < maxRunLines+5; i++ {
		lines = appendOutput(lines, fmt.Sprintf("line %d", i), false)
	}
	if len(lines) != maxRunLines {
		t.Fatalf("len(lines) = %d, want %d", len(lines), maxRunLines)
	}
	if lines[0] != "line 5" {
		t.Errorf("first line = %q, want %q", lines[0], "line 5")
	}
	if last := lines[maxRunLines-1]; last != fmt.Sprintf("line %d", maxRunLines+4) {
		t.Errorf("last line = %q, want %q", last, fmt.Sprintf("line %d", maxRunLines+4))
	}

	lines = appendOutput(lines, "progress", true)
	if len(lines) != maxRunLines {
		t.Errorf("overwrite grew output to %d lines, want %d", len(lines), maxRunLines)
	}
	if lines[maxRunLines-1] != "progress" {
		t.Errorf("overwrite left %q, want %q", lines[maxRunLines-1], "progress")
	}
}

func TestFilterPackages(t *testing.T) {
	pkgs := []manager.Package{
		{Name: "react", Version: "18.2.0"},
		{Name: "react-dom", Version: "18.2.0"},
		{Name: "@types/node", Version: "20.1.0"},
		{Name: "zod", Version: "3.22.4"},
	}
	tests := []struct {
		name  string
		query string
		want  []manager.Package
	}{
		{"empty query keeps everything", "", pkgs},
		{"case insensitive substring", "REACT", pkgs[:2]},
		{"scoped package", "types", pkgs[2:3]},
		{"trims surrounding space", "  zod ", pkgs[3:]},
		{"no match", "nothing", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterPackages(pkgs, tt.query)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("filterPackages(%q) = %#v, want %#v", tt.query, got, tt.want)
			}
		})
	}
}

func TestUpgradeTargets(t *testing.T) {
	upgradable := func(name string, outdated []string) manager.Manager {
		return manager.Manager{Name: name, Kind: "system", Outdated: outdated, UpgradeAll: []string{name, "upgrade"}}
	}
	mgrs := []manager.Manager{
		upgradable("dnf", []string{"dnf", "repoquery"}),
		upgradable("flatpak", []string{"flatpak", "remote-ls"}),
		upgradable("apk", nil),
		upgradable("brew", []string{"brew", "outdated"}),
		{Name: "npm", Kind: "project", Outdated: []string{"npm", "outdated"}, UpgradeAll: []string{"npm", "update"}},
	}
	cache := map[string][]manager.Package{
		"dnf":     make([]manager.Package, 5),
		"flatpak": {},
	}

	targets, pending := upgradeTargets(mgrs, cache)
	if len(targets) != 1 || targets[0].Name != "dnf" {
		t.Errorf("targets = %#v, want [dnf]", targets)
	}
	if !reflect.DeepEqual(pending, []string{"brew"}) {
		t.Errorf("pending = %#v, want [brew]", pending)
	}
}

func TestPanelAt(t *testing.T) {
	m := New("/tmp", []manager.Manager{
		{Name: "dnf", Kind: "system", Outdated: []string{"dnf", "repoquery"}},
		{Name: "npm", Kind: "project"},
	})
	m.width, m.height = 100, 30
	m.resize()
	l := m.layout()

	m.packages = []manager.Package{{Name: "bash", Version: "5.2"}}
	m.filtered = m.packages
	m.updates = []manager.Package{{Name: "foo", Version: "2.0"}}
	m.mCursor, m.pCursor, m.updCursor = 0, 0, 0

	tests := []struct {
		name  string
		x, y  int
		panel int
		row   int
		ok    bool
	}{
		{"managers first row", 1, 2, focusManagers, 0, true},
		{"packages first row", 1, l.mgrH + 2, focusPackages, 0, true},
		{"updates first row", 1, l.mgrH + l.pkgH + 2, focusUpdates, 0, true},
		{"details body", l.leftW + 2, 5, focusDetails, 3, true},
		{"panel title", 1, 1, focusManagers, -1, true},
		{"panel border", 0, 2, focusManagers, 0, false},
		{"details border", l.leftW, 5, focusDetails, 0, false},
		{"status bar", 1, m.height - 1, focusManagers, 0, false},
		{"outside", m.width, 2, focusManagers, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			panel, row, ok := m.panelAt(tt.x, tt.y)
			if panel != tt.panel || row != tt.row || ok != tt.ok {
				t.Errorf("panelAt(%d, %d) = (%d, %d, %v), want (%d, %d, %v)",
					tt.x, tt.y, panel, row, ok, tt.panel, tt.row, tt.ok)
			}
		})
	}
}

func TestPanelAtMapsScroll(t *testing.T) {
	m := New("/tmp", []manager.Manager{{Name: "apt", Kind: "system"}})
	m.width, m.height = 100, 30
	m.resize()
	l := m.layout()

	m.filtered = make([]manager.Package, 40)
	for i := range m.filtered {
		m.filtered[i] = manager.Package{Name: "pkg"}
	}
	m.pCursor = 20
	visible := l.pkgH - 3
	want := scrollStart(m.pCursor, len(m.filtered), visible)

	panel, row, ok := m.panelAt(1, l.mgrH+2)
	if !ok || panel != focusPackages || row != want {
		t.Errorf("panelAt first package row = (%d, %d, %v), want (%d, %d, true)",
			panel, row, ok, focusPackages, want)
	}
}
