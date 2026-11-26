package manager

import (
	"path/filepath"
	"testing"
)

func TestTrust(t *testing.T) {
	f := filepath.Join(t.TempDir(), "sub", "trusted.json")
	old := trustFile
	trustFile = func() string { return f }
	defer func() { trustFile = old }()

	d := t.TempDir()
	if IsTrusted(d) {
		t.Fatal("trusted before Trust")
	}
	if err := Trust(d); err != nil {
		t.Fatal(err)
	}
	if !IsTrusted(d) {
		t.Fatal("not trusted after Trust")
	}
	ms := []Manager{{Name: "dnf", Kind: "system"}, {Name: "npm", Kind: "project"}}
	if got := SystemOnly(ms); len(got) != 1 || got[0].Name != "dnf" || !HasProject(ms) {
		t.Fatalf("SystemOnly/HasProject wrong: %v", got)
	}
}
