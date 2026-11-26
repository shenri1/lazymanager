// Package config loads ~/.config/lazymanager/config.toml (or
// $XDG_CONFIG_HOME/lazymanager/config.toml) on top of built-in defaults.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	DefaultTheme          = "ayu-dark"
	DefaultSelectionStyle = "bg"
	DefaultManyUpdates    = 30
)

// Config is the whole user configuration. Missing values keep their defaults.
type Config struct {
	Theme      ThemeConfig        `toml:"theme"`
	GUI        GUIConfig          `toml:"gui"`
	Keybinding map[string]KeyList `toml:"keybinding"`
	Managers   ManagersConfig     `toml:"managers"`
}

type ThemeConfig struct {
	Name            string            `toml:"name"`
	PaintBackground bool              `toml:"paint_background"`
	Colors          map[string]string `toml:"colors"`
}

type GUIConfig struct {
	NerdFonts      bool   `toml:"nerd_fonts"`
	SelectionStyle string `toml:"selection_style"`
}

type ManagersConfig struct {
	Disabled             []string `toml:"disabled"`
	ManyUpdatesThreshold int      `toml:"many_updates_threshold"`
}

// KeyList is one key or a list of keys. Values in config.toml may be either a
// string or an array of strings.
type KeyList []string

func (k *KeyList) UnmarshalTOML(value any) error {
	switch v := value.(type) {
	case string:
		*k = KeyList{v}
	case []string:
		*k = KeyList(v)
	case []any:
		out := make(KeyList, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return fmt.Errorf("keybinding values must be strings")
			}
			out = append(out, s)
		}
		*k = out
	default:
		return fmt.Errorf("keybinding must be a string or an array of strings")
	}
	return nil
}

// actionOrder is the canonical precedence used to resolve key conflicts.
var actionOrder = []string{
	"quit",
	"help",
	"refresh",
	"up",
	"down",
	"page_up",
	"page_down",
	"top",
	"bottom",
	"next_panel",
	"prev_panel",
	"focus_managers",
	"focus_packages",
	"focus_updates",
	"focus_details",
	"filter",
	"search",
	"install",
	"remove",
	"upgrade",
	"upgrade_all",
	"upgrade_everything",
	"info",
}

var defaultBindings = map[string]KeyList{
	"quit":               {"q", "ctrl+c"},
	"help":               {"?"},
	"refresh":            {"r"},
	"up":                 {"k", "up"},
	"down":               {"j", "down"},
	"page_up":            {"pgup"},
	"page_down":          {"pgdown"},
	"top":                {"g"},
	"bottom":             {"G"},
	"next_panel":         {"tab", "l", "right"},
	"prev_panel":         {"shift+tab", "h", "left"},
	"focus_managers":     {"1"},
	"focus_packages":     {"2"},
	"focus_updates":      {"3"},
	"focus_details":      {"4"},
	"filter":             {"/"},
	"search":             {"s"},
	"install":            {"i"},
	"remove":             {"d"},
	"upgrade":            {"u"},
	"upgrade_all":        {"U"},
	"upgrade_everything": {"A"},
	"info":               {"enter"},
}

// Default returns the built-in configuration.
func Default() Config {
	keys := make(map[string]KeyList, len(defaultBindings))
	for name, list := range defaultBindings {
		keys[name] = slices.Clone(list)
	}
	return Config{
		Theme: ThemeConfig{
			Name:   DefaultTheme,
			Colors: map[string]string{},
		},
		GUI: GUIConfig{
			SelectionStyle: DefaultSelectionStyle,
		},
		Keybinding: keys,
		Managers: ManagersConfig{
			Disabled:             []string{},
			ManyUpdatesThreshold: DefaultManyUpdates,
		},
	}
}

// Actions lists the configurable action names.
func Actions() []string {
	return slices.Clone(actionOrder)
}

// Dir returns the lazymanager configuration directory. XDG_CONFIG_HOME is
// honored on every platform, as is common for terminal tools.
func Dir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "lazymanager")
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "lazymanager")
}

func Path() string {
	dir := Dir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "config.toml")
}

func ThemesDir() string {
	dir := Dir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "themes")
}

// Load reads the default config path. A missing file is not an error.
func Load() (Config, []string) {
	return LoadFile(Path())
}

// LoadFile reads path on top of the defaults. Invalid TOML falls back to the
// defaults; invalid fields fall back individually. Warnings are meant for the
// status bar and never prevent startup.
func LoadFile(path string) (Config, []string) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, []string{"config: " + err.Error()}
	}

	md, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return Default(), []string{"config: " + err.Error() + " — using defaults"}
	}

	var warnings []string
	for _, key := range md.Undecoded() {
		warnings = append(warnings, "config: unknown key "+key.String())
	}
	warnings = append(warnings, cfg.normalize()...)
	return cfg, warnings
}

func (c *Config) normalize() []string {
	var warnings []string

	c.Theme.Name = strings.TrimSpace(c.Theme.Name)
	if c.Theme.Name == "" {
		c.Theme.Name = DefaultTheme
		warnings = append(warnings, "config: theme.name is empty; using "+DefaultTheme)
	}
	if c.Theme.Colors == nil {
		c.Theme.Colors = map[string]string{}
	}

	c.GUI.SelectionStyle = strings.ToLower(strings.TrimSpace(c.GUI.SelectionStyle))
	switch c.GUI.SelectionStyle {
	case "bg", "reverse":
	default:
		warnings = append(warnings, fmt.Sprintf("config: unknown selection_style %q; using %s", c.GUI.SelectionStyle, DefaultSelectionStyle))
		c.GUI.SelectionStyle = DefaultSelectionStyle
	}

	if c.Managers.ManyUpdatesThreshold <= 0 {
		warnings = append(warnings, fmt.Sprintf("config: many_updates_threshold must be positive; using %d", DefaultManyUpdates))
		c.Managers.ManyUpdatesThreshold = DefaultManyUpdates
	}

	disabled := make([]string, 0, len(c.Managers.Disabled))
	for _, name := range c.Managers.Disabled {
		name = strings.TrimSpace(name)
		if name != "" && !slices.Contains(disabled, name) {
			disabled = append(disabled, name)
		}
	}
	c.Managers.Disabled = disabled

	warnings = append(warnings, c.normalizeKeybindings()...)
	return warnings
}

func (c *Config) normalizeKeybindings() []string {
	var warnings []string
	if c.Keybinding == nil {
		c.Keybinding = map[string]KeyList{}
	}

	for name, keys := range c.Keybinding {
		if !slices.Contains(actionOrder, name) {
			warnings = append(warnings, fmt.Sprintf("config: unknown keybinding action %q", name))
			delete(c.Keybinding, name)
			continue
		}
		clean := make(KeyList, 0, len(keys))
		for _, key := range keys {
			key = strings.TrimSpace(key)
			if key == "" {
				warnings = append(warnings, fmt.Sprintf("config: %s has an empty key", name))
				continue
			}
			clean = append(clean, key)
		}
		if len(clean) == 0 {
			warnings = append(warnings, fmt.Sprintf("config: %s has no keys; using defaults", name))
			clean = slices.Clone(defaultBindings[name])
		}
		c.Keybinding[name] = clean
	}
	for _, name := range actionOrder {
		if _, ok := c.Keybinding[name]; !ok {
			c.Keybinding[name] = slices.Clone(defaultBindings[name])
		}
	}

	used := make(map[string]string, len(c.Keybinding))
	for _, name := range actionOrder {
		keys := c.Keybinding[name]
		kept := make(KeyList, 0, len(keys))
		for _, key := range keys {
			if other, ok := used[key]; ok {
				if other == name {
					continue
				}
				warnings = append(warnings, fmt.Sprintf("config: key %q is bound to %s and %s; keeping %s", key, other, name, other))
				continue
			}
			used[key] = name
			kept = append(kept, key)
		}
		if len(kept) == 0 {
			for _, key := range defaultBindings[name] {
				if _, ok := used[key]; ok {
					continue
				}
				used[key] = name
				kept = append(kept, key)
			}
			if len(kept) > 0 {
				warnings = append(warnings, fmt.Sprintf("config: %s has only conflicting keys; using defaults", name))
			}
		}
		if len(kept) == 0 {
			warnings = append(warnings, fmt.Sprintf("config: %s has no usable key", name))
		}
		c.Keybinding[name] = kept
	}
	return warnings
}

// IsManagerDisabled reports whether name is listed in [managers].disabled,
// ignoring case and surrounding spaces.
func (c Config) IsManagerDisabled(name string) bool {
	for _, disabled := range c.Managers.Disabled {
		if strings.EqualFold(strings.TrimSpace(disabled), name) {
			return true
		}
	}
	return false
}

// DefaultTOML is the commented configuration printed by --print-config.
const DefaultTOML = `# lazymanager configuration
# Location: $XDG_CONFIG_HOME/lazymanager/config.toml
#           (~/.config/lazymanager/config.toml by default)

[theme]
# ayu-dark | ayu-mirage | ayu-light | terminal | <user theme>
name = "ayu-dark"
# paint the theme background instead of keeping the terminal's
paint_background = false

# Override individual colors of the selected theme:
# [theme.colors]
# accent = "#E6B450"
# error  = "#F07178"

[gui]
# Use Nerd Font glyphs in selection, panel titles and badges.
nerd_fonts = false
# bg | reverse
selection_style = "bg"

[keybinding]
# Each action accepts one key or a list of keys.
quit = ["q", "ctrl+c"]
help = "?"
refresh = "r"
up = ["k", "up"]
down = ["j", "down"]
page_up = "pgup"
page_down = "pgdown"
top = "g"
bottom = "G"
next_panel = ["tab", "l", "right"]
prev_panel = ["shift+tab", "h", "left"]
focus_managers = "1"
focus_packages = "2"
focus_updates = "3"
focus_details = "4"
filter = "/"
search = "s"
install = "i"
remove = "d"
upgrade = "u"
upgrade_all = "U"
upgrade_everything = "A"
info = "enter"

[managers]
# Managers hidden from the UI, e.g. ["flatpak", "brew"].
disabled = []
# Pending updates at which the yellow warning starts.
many_updates_threshold = 30
`
