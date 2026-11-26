package config

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestMissingFileUsesDefaults(t *testing.T) {
	cfg, warnings := LoadFile(filepath.Join(t.TempDir(), "missing.toml"))
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	if cfg.Theme.Name != DefaultTheme || cfg.GUI.SelectionStyle != DefaultSelectionStyle {
		t.Errorf("cfg = %#v", cfg)
	}
	if cfg.Managers.ManyUpdatesThreshold != DefaultManyUpdates {
		t.Errorf("threshold = %d", cfg.Managers.ManyUpdatesThreshold)
	}
}

func TestLoadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	body := `[theme]
name = "ayu-light"
paint_background = true

[theme.colors]
accent = "#C792EA"

[gui]
nerd_fonts = true
selection_style = "reverse"

[keybinding]
quit = "x"
up = ["w", "up"]

[managers]
disabled = [" flatpak ", "brew", ""]
many_updates_threshold = 5
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, warnings := LoadFile(path)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}

	if cfg.Theme.Name != "ayu-light" || !cfg.Theme.PaintBackground {
		t.Errorf("theme = %#v", cfg.Theme)
	}
	if cfg.Theme.Colors["accent"] != "#C792EA" {
		t.Errorf("colors = %#v", cfg.Theme.Colors)
	}
	if !cfg.GUI.NerdFonts || cfg.GUI.SelectionStyle != "reverse" {
		t.Errorf("gui = %#v", cfg.GUI)
	}
	if !reflect.DeepEqual(cfg.Keybinding["quit"], KeyList{"x"}) {
		t.Errorf("quit = %v", cfg.Keybinding["quit"])
	}
	if !reflect.DeepEqual(cfg.Keybinding["up"], KeyList{"w", "up"}) {
		t.Errorf("up = %v", cfg.Keybinding["up"])
	}
	if !reflect.DeepEqual(cfg.Keybinding["refresh"], KeyList{"r"}) {
		t.Errorf("refresh should keep its default, got %v", cfg.Keybinding["refresh"])
	}
	if len(cfg.Keybinding) != len(Actions()) {
		t.Errorf("keybindings = %d actions, want %d", len(cfg.Keybinding), len(Actions()))
	}
	if !reflect.DeepEqual(cfg.Managers.Disabled, []string{"flatpak", "brew"}) {
		t.Errorf("disabled = %#v", cfg.Managers.Disabled)
	}
	if cfg.Managers.ManyUpdatesThreshold != 5 {
		t.Errorf("threshold = %d", cfg.Managers.ManyUpdatesThreshold)
	}
}

func TestInvalidTOMLFallsBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[theme\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, warnings := LoadFile(path)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "using defaults") {
		t.Fatalf("warnings = %v", warnings)
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Errorf("cfg = %#v, want defaults", cfg)
	}
}

func TestInvalidFieldsFallBackIndividually(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	body := `[gui]
selection_style = "rainbow"

[managers]
many_updates_threshold = 0

[keybinding]
nope = "z"
quit = ""
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, warnings := LoadFile(path)
	joined := strings.Join(warnings, "\n")
	for _, want := range []string{"selection_style", "many_updates_threshold", "unknown keybinding action", "no keys"} {
		if !strings.Contains(joined, want) {
			t.Errorf("warnings missing %q:\n%s", want, joined)
		}
	}
	if cfg.GUI.SelectionStyle != DefaultSelectionStyle {
		t.Errorf("selection_style = %q", cfg.GUI.SelectionStyle)
	}
	if cfg.Managers.ManyUpdatesThreshold != DefaultManyUpdates {
		t.Errorf("threshold = %d", cfg.Managers.ManyUpdatesThreshold)
	}
	if _, ok := cfg.Keybinding["nope"]; ok {
		t.Error("unknown action was kept")
	}
	if !reflect.DeepEqual(cfg.Keybinding["quit"], defaultBindings["quit"]) {
		t.Errorf("quit = %v, want defaults", cfg.Keybinding["quit"])
	}
}

func TestKeyConflictKeepsFirstAction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[keybinding]\nup = \"j\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, warnings := LoadFile(path)
	if len(warnings) == 0 || !strings.Contains(strings.Join(warnings, "\n"), "bound to") {
		t.Fatalf("warnings = %v", warnings)
	}
	if !reflect.DeepEqual(cfg.Keybinding["up"], KeyList{"j"}) {
		t.Errorf("up = %v", cfg.Keybinding["up"])
	}
	if !reflect.DeepEqual(cfg.Keybinding["down"], KeyList{"down"}) {
		t.Errorf("down = %v, want [down] after dropping the conflict", cfg.Keybinding["down"])
	}
}

func TestUnknownKeyWarns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[gui]\nnerd_font = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, warnings := LoadFile(path)
	if len(warnings) == 0 || !strings.Contains(strings.Join(warnings, "\n"), "unknown key") {
		t.Fatalf("warnings = %v", warnings)
	}
	if cfg.GUI.NerdFonts {
		t.Error("misspelled key should not enable nerd fonts")
	}
}

func TestDefaultTOMLIsValid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(DefaultTOML), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, warnings := LoadFile(path)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Errorf("default TOML did not round-trip:\n got %#v\nwant %#v", cfg, Default())
	}
}

func TestDirHonorsXDG(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if got, want := Dir(), filepath.Join(dir, "lazymanager"); got != want {
		t.Errorf("Dir() = %q, want %q", got, want)
	}
	if got, want := Path(), filepath.Join(dir, "lazymanager", "config.toml"); got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func TestIsManagerDisabled(t *testing.T) {
	cfg := Default()
	cfg.Managers.Disabled = []string{"Flatpak", " brew "}
	for _, name := range []string{"flatpak", "FLATPAK", "brew"} {
		if !cfg.IsManagerDisabled(name) {
			t.Errorf("%q should be disabled", name)
		}
	}
	if cfg.IsManagerDisabled("apt") {
		t.Error("apt should not be disabled")
	}
}

func TestActionsMatchDefaults(t *testing.T) {
	for _, name := range Actions() {
		if _, ok := defaultBindings[name]; !ok {
			t.Errorf("action %q has no defaults", name)
		}
	}
	for name := range defaultBindings {
		if !slices.Contains(Actions(), name) {
			t.Errorf("default %q is not in Actions()", name)
		}
	}
}
