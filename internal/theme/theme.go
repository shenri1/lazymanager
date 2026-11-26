// Package theme holds the semantic color palette used by the TUI. Themes are
// data (TOML), not code, so they can be swapped or extended without a rebuild.
package theme

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

//go:embed themes/*.toml
var builtinFS embed.FS

// Theme maps semantic roles to colors. A color is either "#RRGGBB" or an ANSI
// palette index (0-15). An empty color means "inherit the terminal".
type Theme struct {
	Name           string
	Background     string
	Foreground     string
	Accent         string
	BorderActive   string
	BorderInactive string
	Title          string
	SelectedBg     string
	SelectedFg     string
	Dim            string
	Success        string
	Warning        string
	Error          string
}

type themeFile struct {
	Name    string            `toml:"name"`
	Extends string            `toml:"extends"`
	Colors  map[string]string `toml:"colors"`
}

var builtinNames = []string{"ayu-dark", "ayu-mirage", "ayu-light", "terminal"}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Builtins lists the embedded themes, default first.
func Builtins() []string {
	return append([]string(nil), builtinNames...)
}

// Default returns the built-in ayu-dark theme.
func Default() Theme {
	t, _, err := loadBuiltin("ayu-dark")
	if err != nil {
		panic("theme: embedded default is invalid: " + err.Error())
	}
	return t
}

// Resolve loads a built-in theme or a user theme from dir/<name>.toml. User
// themes may extend another built-in or user theme.
func Resolve(name, dir string) (Theme, error) {
	if name == "" {
		return Default(), nil
	}
	return resolve(name, dir, make(map[string]bool), 0)
}

func resolve(name, dir string, seen map[string]bool, depth int) (Theme, error) {
	if depth > 20 {
		return Theme{}, fmt.Errorf("theme %q: extends chain is too deep", name)
	}
	if !validName(name) {
		return Theme{}, fmt.Errorf("invalid theme name %q", name)
	}
	if seen[name] {
		return Theme{}, fmt.Errorf("theme %q: extends cycle", name)
	}
	seen[name] = true
	defer delete(seen, name)

	if t, ok, err := loadBuiltin(name); ok {
		return t, err
	}
	if dir == "" {
		return Theme{}, fmt.Errorf("unknown theme %q", name)
	}

	path := filepath.Join(dir, name+".toml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Theme{}, fmt.Errorf("unknown theme %q", name)
		}
		return Theme{}, fmt.Errorf("theme %q: %w", name, err)
	}
	var f themeFile
	if _, err := toml.Decode(string(data), &f); err != nil {
		return Theme{}, fmt.Errorf("%s: %w", path, err)
	}

	base := Theme{}
	if f.Extends != "" {
		base, err = resolve(f.Extends, dir, seen, depth+1)
		if err != nil {
			return Theme{}, fmt.Errorf("%s: %w", path, err)
		}
	}
	base.Name = name
	if f.Name != "" {
		base.Name = f.Name
	}
	if err := applyColors(&base, f.Colors); err != nil {
		return Theme{}, fmt.Errorf("%s: %w", path, err)
	}
	return base, nil
}

// WithOverrides applies [theme.colors] on top of t. Invalid roles or colors are
// reported and skipped, so the rest of the overrides still apply.
func (t Theme) WithOverrides(colors map[string]string) (Theme, []error) {
	var errs []error
	for _, role := range sortedKeys(colors) {
		value := strings.TrimSpace(colors[role])
		if err := validColor(value); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", role, err))
			continue
		}
		if err := setColor(&t, role, value); err != nil {
			errs = append(errs, err)
		}
	}
	return t, errs
}

// ListUser returns the names of the user themes in dir (file names without the
// .toml suffix), sorted.
func ListUser(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name, ok := strings.CutSuffix(e.Name(), ".toml")
		if !ok || name == "" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func loadBuiltin(name string) (Theme, bool, error) {
	if !contains(builtinNames, name) {
		return Theme{}, false, nil
	}
	data, err := fs.ReadFile(builtinFS, "themes/"+name+".toml")
	if err != nil {
		return Theme{}, true, fmt.Errorf("theme %q: %w", name, err)
	}
	var f themeFile
	if _, err := toml.Decode(string(data), &f); err != nil {
		return Theme{}, true, fmt.Errorf("theme %q: %w", name, err)
	}
	t := Theme{Name: name}
	if f.Name != "" {
		t.Name = f.Name
	}
	if err := applyColors(&t, f.Colors); err != nil {
		return Theme{}, true, fmt.Errorf("theme %q: %w", name, err)
	}
	return t, true, nil
}

func applyColors(t *Theme, colors map[string]string) error {
	for _, role := range sortedKeys(colors) {
		value := strings.TrimSpace(colors[role])
		if err := validColor(value); err != nil {
			return fmt.Errorf("%s: %w", role, err)
		}
		if err := setColor(t, role, value); err != nil {
			return err
		}
	}
	return nil
}

func setColor(t *Theme, role, value string) error {
	switch role {
	case "background":
		t.Background = value
	case "foreground":
		t.Foreground = value
	case "accent":
		t.Accent = value
	case "border_active":
		t.BorderActive = value
	case "border_inactive":
		t.BorderInactive = value
	case "title":
		t.Title = value
	case "selected_bg":
		t.SelectedBg = value
	case "selected_fg":
		t.SelectedFg = value
	case "dim":
		t.Dim = value
	case "success":
		t.Success = value
	case "warning":
		t.Warning = value
	case "error":
		t.Error = value
	default:
		return fmt.Errorf("unknown color role %q", role)
	}
	return nil
}

func validColor(s string) error {
	if s == "" {
		return nil
	}
	if hexColor.MatchString(s) {
		return nil
	}
	if n, err := strconv.Atoi(s); err == nil && n >= 0 && n <= 15 {
		return nil
	}
	return fmt.Errorf("invalid color %q (want #RRGGBB or 0-15)", s)
}

func validName(name string) bool {
	return name != "" && name != "." && name != ".." &&
		!strings.ContainsAny(name, `/\`) && filepath.Base(name) == name
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
