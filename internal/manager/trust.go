package manager

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
)

// trustFile is a var so tests can point it at a temp file.
// Default: <user config dir>/lazymanager/trusted.json (~/.config on Linux).
var trustFile = func() string {
	d, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(d, "lazymanager", "trusted.json")
}

func trustKey(dir string) string {
	if p, err := filepath.EvalSymlinks(dir); err == nil {
		return p
	}
	return dir
}

func readTrusted() []string {
	var dirs []string
	if f := trustFile(); f != "" {
		if b, err := os.ReadFile(f); err == nil {
			_ = json.Unmarshal(b, &dirs) // corrupt file => nothing trusted
		}
	}
	return dirs
}

// IsTrusted reports whether dir was previously marked trusted.
func IsTrusted(dir string) bool {
	return slices.Contains(readTrusted(), trustKey(dir))
}

// Trust records dir as trusted so project managers may run there.
func Trust(dir string) error {
	f := trustFile()
	if f == "" {
		return os.ErrNotExist
	}
	dirs := readTrusted()
	k := trustKey(dir)
	if slices.Contains(dirs, k) {
		return nil
	}
	b, _ := json.MarshalIndent(append(dirs, k), "", "  ")
	if err := os.MkdirAll(filepath.Dir(f), 0o700); err != nil {
		return err
	}
	return os.WriteFile(f, b, 0o600)
}

// SystemOnly drops project managers.
func SystemOnly(ms []Manager) []Manager {
	return slices.DeleteFunc(slices.Clone(ms), func(m Manager) bool { return m.Kind == "project" })
}

// HasProject reports whether any manager is a project manager.
func HasProject(ms []Manager) bool {
	return slices.ContainsFunc(ms, func(m Manager) bool { return m.Kind == "project" })
}
