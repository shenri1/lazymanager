package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/shenri1/lazymanager/internal/config"
	"github.com/shenri1/lazymanager/internal/manager"
	"github.com/shenri1/lazymanager/internal/theme"
)

func TestEveryConfigActionIsKnownToUI(t *testing.T) {
	if len(actionByName) != len(config.Actions()) {
		t.Errorf("ui knows %d actions, config has %d", len(actionByName), len(config.Actions()))
	}
	for _, name := range config.Actions() {
		if _, ok := actionByName[name]; !ok {
			t.Errorf("config action %q has no UI action", name)
		}
	}
}

func TestKeymapRemap(t *testing.T) {
	cfg := config.Default()
	cfg.Keybinding["quit"] = config.KeyList{"x"}
	km := newKeymap(cfg.Keybinding)

	if got := km.action("x"); got != actQuit {
		t.Errorf("action(x) = %v, want quit", got)
	}
	if got := km.action("q"); got != actNone {
		t.Errorf("action(q) = %v, want none after remap", got)
	}
	if got := km.label(actQuit); got != "x" {
		t.Errorf("label(quit) = %q, want x", got)
	}
}

func TestKeymapMultipleKeys(t *testing.T) {
	cfg := config.Default()
	cfg.Keybinding["refresh"] = config.KeyList{"r", "f5"}
	km := newKeymap(cfg.Keybinding)
	if km.action("f5") != actRefresh || km.action("r") != actRefresh {
		t.Error("both configured keys should refresh")
	}
}

func TestNoColorForcesReverse(t *testing.T) {
	th := theme.Default()
	s := newStyles(th, "bg", false, true, true)
	if !s.selectedFocused.GetReverse() {
		t.Error("NO_COLOR selection should use reverse")
	}
	if s.selectedInactive.GetReverse() {
		t.Error("inactive NO_COLOR selection should not use reverse")
	}
	if s.paint {
		t.Error("NO_COLOR should disable paint_background")
	}

	s = newStyles(th, "bg", false, false, false)
	if got := s.selectedFocused.GetBackground(); got != lipgloss.Color(th.SelectedBg) {
		t.Errorf("selected background = %v, want %v", got, lipgloss.Color(th.SelectedBg))
	}
}

func TestSelectionIsFocusAware(t *testing.T) {
	th := theme.Default()
	s := newStyles(th, "bg", false, false, false)
	if got := s.selection(true).GetBackground(); got != lipgloss.Color(th.SelectedBg) {
		t.Errorf("focused background = %v", got)
	}
	if got := s.selection(false).GetBackground(); got != lipgloss.Color(th.SelectedBg) {
		t.Errorf("inactive background = %v", got)
	}
	if s.selection(true).GetForeground() == s.selection(false).GetForeground() {
		t.Error("focused and inactive selections should differ")
	}
}

func TestReverseSelection(t *testing.T) {
	s := newStyles(theme.Default(), "reverse", false, false, false)
	if !s.selection(true).GetReverse() || !s.selection(false).GetReverse() {
		t.Error("reverse selection should set Reverse")
	}
}

func TestNerdFontIcons(t *testing.T) {
	if got := newStyles(theme.Default(), "bg", false, false, false).icons.pointer; got != "▸" {
		t.Errorf("plain pointer = %q", got)
	}
	if got := newStyles(theme.Default(), "bg", true, false, false).icons.pointer; got == "▸" {
		t.Error("nerd_fonts should replace the pointer")
	}
}

func TestNewShowsConfigWarnings(t *testing.T) {
	m := New("/tmp", nil, Options{Warnings: []string{"config: bad theme", "config: bad key"}})
	if !m.statusWarn || !strings.Contains(m.status, "bad theme") || !strings.Contains(m.status, "bad key") {
		t.Errorf("status = %q warn=%v", m.status, m.statusWarn)
	}
}

func TestManyUpdatesThresholdComesFromConfig(t *testing.T) {
	cfg := config.Default()
	cfg.Managers.ManyUpdatesThreshold = 5
	m := New("/tmp", []manager.Manager{{Name: "dnf", Kind: "system", Outdated: []string{"x"}}}, Options{Config: cfg})
	m.width, m.height = 100, 30
	m.resize()

	pkgs := make([]manager.Package, 5)
	for i := range pkgs {
		pkgs[i] = manager.Package{Name: "pkg"}
	}
	next, _ := m.Update(updatesMsg{manager: "dnf", seq: 0, pkgs: pkgs})
	m = next.(model)
	if !m.statusWarn || !strings.Contains(m.status, "5 updates pending") {
		t.Errorf("status = %q warn=%v", m.status, m.statusWarn)
	}

	m.updates = pkgs
	m.loadUpdates = false
	title, _ := m.updatesTitle()
	if !strings.Contains(title, "⚠") {
		t.Errorf("updates title = %q, want the warning symbol", title)
	}
}

func TestStatusBarSymbols(t *testing.T) {
	m := New("/tmp", nil)
	m.width, m.height = 80, 30
	m.status = "boom"
	m.statusErr = true
	if bar := m.statusBar(); !strings.Contains(bar, "✗") {
		t.Errorf("error status = %q, want ✗", bar)
	}

	m.statusErr = false
	m.statusWarn = true
	if bar := m.statusBar(); !strings.Contains(bar, "⚠") {
		t.Errorf("warning status = %q, want ⚠", bar)
	}

	m.statusWarn = false
	m.status = everythingUpToDate
	if bar := m.statusBar(); !strings.Contains(bar, "✓") {
		t.Errorf("ok status = %q, want ✓", bar)
	}
}

func TestHelpUsesConfiguredKeys(t *testing.T) {
	cfg := config.Default()
	cfg.Keybinding["quit"] = config.KeyList{"x"}
	cfg.Keybinding["refresh"] = config.KeyList{"f5"}
	m := New("/tmp", nil, Options{Config: cfg})
	text := helpText(m.keys)
	for _, want := range []string{"x", "f5"} {
		if !strings.Contains(text, want) {
			t.Errorf("help text missing %q:\n%s", want, text)
		}
	}
}

func TestStatusHintsUseConfiguredKeys(t *testing.T) {
	cfg := config.Default()
	cfg.Keybinding["search"] = config.KeyList{"f"}
	m := New("/tmp", nil, Options{Config: cfg})
	m.width, m.height = 120, 30
	m.resize()
	m.focus = focusManagers
	if bar := m.statusBar(); !strings.Contains(bar, "f search") {
		t.Errorf("status hints = %q, want configured search key", bar)
	}
}

func TestPaintBackground(t *testing.T) {
	th := theme.Default()
	s := newStyles(th, "bg", false, false, true)
	if !s.paint {
		t.Fatal("paint should be enabled")
	}
	if got := s.base.GetBackground(); got != lipgloss.Color(th.Background) {
		t.Errorf("base background = %v, want %v", got, lipgloss.Color(th.Background))
	}
}

func TestViewRendersWithEveryThemeAndOption(t *testing.T) {
	for _, name := range theme.Builtins() {
		th, err := theme.Resolve(name, t.TempDir())
		if err != nil {
			t.Fatalf("theme %q: %v", name, err)
		}
		for _, selection := range []string{"bg", "reverse"} {
			for _, nerd := range []bool{false, true} {
				for _, noColor := range []bool{false, true} {
					cfg := config.Default()
					cfg.GUI.SelectionStyle = selection
					cfg.GUI.NerdFonts = nerd
					cfg.Theme.PaintBackground = true
					m := New("/tmp", []manager.Manager{{Name: "apt", Kind: "system", Outdated: []string{"x"}}},
						Options{Theme: th, Config: cfg, NoColor: noColor, Warnings: []string{"config: warning"}})
					m.width, m.height = 100, 30
					m.resize()
					m.packages = []manager.Package{{Name: "bash", Version: "5.2"}, {Name: "日本語パッケージ", Version: "1.0"}}
					m.filtered = m.packages
					m.updates = m.packages
					m.setDetails("Details", "some text")
					lines := strings.Split(m.View(), "\n")
					if len(lines) > m.height {
						t.Errorf("%s/%s/nerd=%v/nocolor=%v: %d lines > %d", name, selection, nerd, noColor, len(lines), m.height)
					}
					for i, line := range lines {
						if width := ansi.StringWidth(line); width > m.width {
							t.Errorf("%s/%s/nerd=%v/nocolor=%v: line %d width %d > %d", name, selection, nerd, noColor, i, width, m.width)
						}
					}
				}
			}
		}
	}
}
