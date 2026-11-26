package manager

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseList(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []Package
	}{
		{
			name: "name version",
			in:   "bash 5.2.15\nzlib 1.2.13\n",
			want: []Package{{Name: "bash", Version: "5.2.15"}, {Name: "zlib", Version: "1.2.13"}},
		},
		{
			name: "npm tree",
			in:   "├── react@18.2.0\n└── zod@3.22.4\n",
			want: []Package{{Name: "react", Version: "18.2.0"}, {Name: "zod", Version: "3.22.4"}},
		},
		{
			name: "scoped package",
			in:   "├── @types/node@20.1.0\n",
			want: []Package{{Name: "@types/node", Version: "20.1.0"}},
		},
		{
			name: "pip freeze",
			in:   "requests==2.31.0\nflask==3.0.0\n",
			want: []Package{{Name: "requests", Version: "2.31.0"}, {Name: "flask", Version: "3.0.0"}},
		},
		{
			name: "headers blanks and comments",
			in:   "Listing...\nInstalled packages:\n# a comment\n\nfoo 1.0\nbar\n",
			want: []Package{{Name: "foo", Version: "1.0"}, {Name: "bar"}},
		},
		{
			name: "at version without separate field",
			in:   "foo@1.2\n",
			want: []Package{{Name: "foo", Version: "1.2"}},
		},
		{
			name: "npm root line and empty tree",
			in:   "x@1.0.0 /home/u/x\n└── (empty)\n",
			want: nil,
		},
		{
			name: "tab separated",
			in:   "org.gnome.Builder\t48.0\n",
			want: []Package{{Name: "org.gnome.Builder", Version: "48.0"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseList(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseList(%q) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseOutdated(t *testing.T) {
	tests := []struct {
		name string
		in   string
		col  int
		want []Package
	}{
		{
			name: "dnf style",
			in:   "code 1.148.0-1\n",
			col:  1,
			want: []Package{{Name: "code", Version: "1.148.0-1"}},
		},
		{
			name: "pacman arrow",
			in:   "foo 1.0-1 -> 1.1-1\n",
			col:  0,
			want: []Package{{Name: "foo", Version: "1.1-1"}},
		},
		{
			name: "brew less-than",
			in:   "wget (1.21) < 1.24\n",
			col:  0,
			want: []Package{{Name: "wget", Version: "1.24"}},
		},
		{
			name: "npm table with header col 3",
			in: "Package    Current  Wanted  Latest  Location\n" +
				"lodash     4.17.20  4.17.21  4.17.21  node_modules/lodash\n",
			col:  3,
			want: []Package{{Name: "lodash", Version: "4.17.21"}},
		},
		{
			name: "pip columns with separator col 2",
			in: "Package Version Latest Type\n" +
				"------- ------- ------ -----\n" +
				"flask   2.0.0   2.3.0  wheel\n",
			col:  2,
			want: []Package{{Name: "flask", Version: "2.3.0"}},
		},
		{
			name: "col zero means field one",
			in:   "foo 1.2\n",
			col:  0,
			want: []Package{{Name: "foo", Version: "1.2"}},
		},
		{
			name: "skips noise and tree chars",
			in:   "Listing...\n# comment\n| row\n= row\n- separator\n└── bar 2.0\n",
			col:  1,
			want: []Package{{Name: "bar", Version: "2.0"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseOutdated(tt.in, tt.col)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseOutdated(%q, %d) = %#v, want %#v", tt.in, tt.col, got, tt.want)
			}
		})
	}
}

func TestUpdatesIgnoresExitErrorWithOutput(t *testing.T) {
	needSh(t)
	m := Manager{
		Name:        "test",
		Bin:         "sh",
		Outdated:    []string{"sh", "-c", `printf 'zod 1.0\nbash 2.0\n'; exit 1`},
		OutdatedCol: 1,
	}
	got, err := m.Updates()
	if err != nil {
		t.Fatalf("Updates() error = %v, want nil for non-empty output with exit 1", err)
	}
	want := []Package{{Name: "bash", Version: "2.0"}, {Name: "zod", Version: "1.0"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Updates() = %#v, want %#v", got, want)
	}
}

func needSh(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not on PATH")
	}
}

func TestUpdatesOKExitCode(t *testing.T) {
	needSh(t)
	m := Manager{Name: "test", Outdated: []string{"sh", "-c", "exit 1"}, OutdatedOK: []int{1}}
	if got, err := m.Updates(); err != nil || len(got) != 0 {
		t.Errorf("Updates() = %v, %v; want empty, nil for allowed exit 1", got, err)
	}
	m.OutdatedOK = nil
	if _, err := m.Updates(); err == nil {
		t.Error("Updates() error = nil, want error for exit 1 not in OutdatedOK")
	}
}

func TestUpdatesUnsupported(t *testing.T) {
	if _, err := (Manager{Name: "test"}).Updates(); err == nil {
		t.Error("Updates() error = nil, want error for a manager without Outdated")
	}
}

func TestDetect(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"go.mod", "pnpm-lock.yaml", "package.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	old := lookPath
	t.Cleanup(func() { lookPath = old })
	lookPath = func(file string) (string, error) {
		switch file {
		case "go", "pnpm", "npm":
			return "/usr/bin/" + file, nil
		}
		return "", os.ErrNotExist
	}

	var got []string
	for _, m := range Detect(dir) {
		got = append(got, m.Name)
	}
	want := []string{"go", "pnpm"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Detect() = %v, want %v (npm must be skipped when a pnpm lockfile exists)", got, want)
	}
}

func TestCmdSudo(t *testing.T) {
	m := Manager{Name: "test", Kind: "system", Bin: "echo", Sudo: true, Install: []string{"echo", "install"}}

	t.Run("mutate with sudo", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("running as root, sudo is not prefixed")
		}
		got := m.Cmd(true, m.Install, "pkg").Args
		want := []string{"sudo", "-n", "echo", "install", "pkg"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Args = %v, want %v", got, want)
		}
	})

	t.Run("rejects option-like arg", func(t *testing.T) {
		if err := m.Cmd(true, m.Install, "--index-url=x").Run(); err == nil {
			t.Error("expected error for arg starting with '-'")
		}
	})

	t.Run("non-mutate", func(t *testing.T) {
		got := m.Cmd(false, m.Install, "pkg").Args
		want := []string{"echo", "install", "pkg"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Args = %v, want %v", got, want)
		}
	})

	t.Run("no sudo flag", func(t *testing.T) {
		plain := Manager{Name: "test", Kind: "project", Bin: "echo", Install: []string{"echo", "install"}}
		got := plain.Cmd(true, plain.Install, "pkg").Args
		want := []string{"echo", "install", "pkg"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Args = %v, want %v", got, want)
		}
	})
}

func TestParseListEdgeCases(t *testing.T) {
	cases := []struct {
		name, in string
		want     []Package
	}{
		{"cargo root, section, dupes",
			"myproj v0.1.0 (/home/x)\nserde v1.0.0\nserde v1.0.0 (*)\n\n[dev-dependencies]\ntokio v1.0.0\n",
			[]Package{{"serde", "v1.0.0"}, {"tokio", "v1.0.0"}}},
		{"pip direct reference", "foo @ file:///tmp/foo\nbar==1.2\n",
			[]Package{{"foo", "file:///tmp/foo"}, {"bar", "1.2"}}},
	}
	for _, c := range cases {
		if got := ParseList(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: ParseList = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestParseOutdatedBrewCask(t *testing.T) {
	got := ParseOutdated("foo (1.0) != 1.1\n", 1)
	if want := []Package{{"foo", "1.1"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("ParseOutdated = %v, want %v", got, want)
	}
}
