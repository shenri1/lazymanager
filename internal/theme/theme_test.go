package theme

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBuiltinsLoad(t *testing.T) {
	for _, name := range Builtins() {
		th, err := Resolve(name, t.TempDir())
		if err != nil {
			t.Errorf("Resolve(%q): %v", name, err)
			continue
		}
		if th.Name == "" {
			t.Errorf("Resolve(%q): empty name", name)
		}
		if name != "terminal" && th.BorderActive == "" {
			t.Errorf("Resolve(%q): missing border_active", name)
		}
	}
}

func TestDefaultIsAyuDark(t *testing.T) {
	if got := Default().Name; got != "ayu-dark" {
		t.Errorf("Default().Name = %q, want ayu-dark", got)
	}
}

func TestUserThemeExtendsUserTheme(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "base", `name = "base"
[colors]
accent = "#112233"
error = "#445566"
`)
	writeTheme(t, dir, "child", `name = "child"
extends = "base"
[colors]
accent = "#AABBCC"
`)

	th, err := Resolve("child", dir)
	if err != nil {
		t.Fatal(err)
	}
	if th.Name != "child" || th.Accent != "#AABBCC" || th.Error != "#445566" {
		t.Errorf("child = %#v", th)
	}
}

func TestUserThemeExtendsBuiltin(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "mine", `name = "mine"
extends = "ayu-dark"
[colors]
accent = "#C792EA"
`)

	th, err := Resolve("mine", dir)
	if err != nil {
		t.Fatal(err)
	}
	if th.Accent != "#C792EA" {
		t.Errorf("accent = %q, want #C792EA", th.Accent)
	}
	if th.Background != Default().Background {
		t.Errorf("background = %q, want inherited %q", th.Background, Default().Background)
	}
}

func TestInvalidUserColor(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "bad", `name = "bad"
[colors]
accent = "red"
`)
	if _, err := Resolve("bad", dir); err == nil {
		t.Fatal("expected an error for an invalid color")
	}
}

func TestUnknownColorRole(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "bad", `name = "bad"
[colors]
nope = "#112233"
`)
	if _, err := Resolve("bad", dir); err == nil {
		t.Fatal("expected an error for an unknown color role")
	}
}

func TestUnknownTheme(t *testing.T) {
	if _, err := Resolve("does-not-exist", t.TempDir()); err == nil {
		t.Fatal("expected an error for an unknown theme")
	}
}

func TestExtendsCycle(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "a", "name = \"a\"\nextends = \"b\"\n")
	writeTheme(t, dir, "b", "name = \"b\"\nextends = \"a\"\n")
	if _, err := Resolve("a", dir); err == nil {
		t.Fatal("expected an error for an extends cycle")
	}
}

func TestThemeNameCannotEscapeThemesDir(t *testing.T) {
	if _, err := Resolve("../secret", t.TempDir()); err == nil {
		t.Fatal("expected an error for a path-like theme name")
	}
}

func TestWithOverrides(t *testing.T) {
	th := Default()
	got, errs := th.WithOverrides(map[string]string{
		"accent": "#C792EA",
		"error":  "nope",
		"nope":   "#112233",
	})
	if got.Accent != "#C792EA" {
		t.Errorf("accent = %q, want #C792EA", got.Accent)
	}
	if got.Error != th.Error {
		t.Errorf("error changed to %q despite invalid color", got.Error)
	}
	if len(errs) != 2 {
		t.Errorf("errs = %v, want 2", errs)
	}
}

func TestListUser(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "bbb", "name = \"bbb\"\n")
	writeTheme(t, dir, "aaa", "name = \"aaa\"\n")
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := ListUser(dir)
	want := []string{"aaa", "bbb"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ListUser = %v, want %v", got, want)
	}
}

func TestTerminalColors(t *testing.T) {
	th, err := Resolve("terminal", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for role, value := range map[string]string{
		"accent": th.Accent, "border_active": th.BorderActive, "error": th.Error,
	} {
		if strings.HasPrefix(value, "#") || value == "" {
			t.Errorf("%s = %q, want an ANSI index", role, value)
		}
	}
}

func writeTheme(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".toml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
