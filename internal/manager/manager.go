package manager

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// Manager describes one package manager. Every supported manager is a value in
// Table; behaviour comes from the fields, not from a per-manager type.
type Manager struct {
	Name        string   // "dnf", "npm", ...
	Kind        string   // "system" or "project"
	Bin         string   // executable looked up with exec.LookPath
	Sudo        bool     // prefix mutating commands with "sudo"
	List        []string // lists installed pkgs; output parsed by ParseList
	Search      []string // query is appended as last arg; nil = unsupported
	Install     []string // pkg names appended
	Remove      []string
	Upgrade     []string // pkg names appended
	UpgradeAll  []string
	Info        []string // pkg name appended
	Detect      []string // project kind only: any of these files in cwd => applies
	Outdated    []string // lists available updates; nil = unsupported
	OutdatedCol int      // field index holding the NEW version; 0 means 1
}

type Package struct {
	Name    string
	Version string
}

// lookPath is a variable so tests can stub executable discovery.
var lookPath = exec.LookPath

// jsLockfiles are the lockfiles that mean npm should not also be offered.
var jsLockfiles = []string{"pnpm-lock.yaml", "yarn.lock", "bun.lockb", "bun.lock"}

// Detect returns available system managers followed by applicable project
// managers. A project manager applies when one of its Detect files exists in
// cwd and its binary is on PATH.
func Detect(cwd string) []Manager {
	var found []Manager
	otherJS := false
	for _, name := range jsLockfiles {
		if fileExists(filepath.Join(cwd, name)) {
			otherJS = true
			break
		}
	}
	for _, m := range Table {
		if _, err := lookPath(m.Bin); err != nil {
			continue
		}
		if m.Kind == "project" {
			if m.Name == "npm" && otherJS {
				continue
			}
			if !anyFileExists(cwd, m.Detect) {
				continue
			}
		}
		found = append(found, m)
	}
	return found
}

func anyFileExists(dir string, names []string) bool {
	for _, name := range names {
		if fileExists(filepath.Join(dir, name)) {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// NeedsSudo reports whether mutating commands of m must be prefixed with sudo.
func NeedsSudo(m Manager) bool {
	return m.Sudo && os.Geteuid() != 0
}

// Cmd builds the command for an action. action[0] is the program (it may
// differ from Bin, e.g. apt lists via dpkg-query). Mutating actions are
// prefixed with sudo when the manager asks for it and we are not root. The
// prefix is non-interactive (-n); the password, when needed, is obtained by
// the TUI beforehand through SudoAuth.
func (m Manager) Cmd(mutate bool, action []string, args ...string) *exec.Cmd {
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			c := exec.Command("false")
			c.Err = fmt.Errorf("refusing argument starting with '-': %q", a)
			return c
		}
	}
	argv :=append(append([]string{}, action...), args...)
	if mutate && NeedsSudo(m) {
		argv = append([]string{"sudo", "-n"}, argv...)
	}
	return exec.Command(argv[0], argv[1:]...)
}

// SudoCached reports whether sudo can run without a password right now
// (cached credentials or NOPASSWD). Runs `sudo -n true`.
func SudoCached() bool {
	return exec.Command("sudo", "-n", "true").Run() == nil
}

// SudoAuth validates password with `sudo -S -v -p ""`, writing password+"\n"
// to stdin. Returns a clear error on wrong password (use stderr text).
func SudoAuth(password string) error {
	c := exec.Command("sudo", "-S", "-v", "-p", "")
	c.Stdin = strings.NewReader(password + "\n")
	var stderr strings.Builder
	c.Stderr = &stderr
	if err := c.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("sudo: %s", msg)
	}
	return nil
}

// Installed lists the manager's installed packages, sorted by name.
func (m Manager) Installed() ([]Package, error) {
	out, err := Run(m.Cmd(false, m.List))
	if err != nil {
		return nil, fmt.Errorf("%w\n%s", err, out)
	}
	pkgs := ParseList(out)
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs, nil
}

// Updates lists the package manager's available updates, sorted by name.
func (m Manager) Updates() ([]Package, error) {
	if len(m.Outdated) == 0 {
		return nil, fmt.Errorf("%s does not support updates", m.Name)
	}
	out, err := Run(m.Cmd(false, m.Outdated))
	if err != nil {
		// Some commands (notably "npm outdated") exit non-zero when updates
		// exist. Keep parsing whenever there is usable output.
		if _, ok := err.(*exec.ExitError); !ok || strings.TrimSpace(out) == "" {
			return nil, fmt.Errorf("%w\n%s", err, out)
		}
	}
	pkgs := ParseOutdated(out, m.OutdatedCol)
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs, nil
}

// ansiEscape matches ANSI escape sequences: CSI, OSC (BEL or ST terminated),
// and two-byte escapes.
var ansiEscape = regexp.MustCompile(`\x1b(?:\[[0-9;?]*[ -/]*[@-~]|\][^\x07\x1b]*(?:\x07|\x1b\\)?|[@-_])`)

// Sanitize strips escape sequences and control characters (except \n and \t)
// from untrusted program output so it cannot drive the terminal.
func Sanitize(s string) string {
	s = ansiEscape.ReplaceAllString(s, "")
	return strings.Map(func(r rune) rune {
		if r != '\n' && r != '\t' && unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

// Run returns sanitized stdout, plus stderr when the command fails, so errors
// say why. Output is untrusted (package descriptions come from the internet).
func Run(c *exec.Cmd) (string, error) {
	out, err := c.Output()
	if ee, ok := err.(*exec.ExitError); ok {
		out = append(out, ee.Stderr...)
	}
	return Sanitize(string(out)), err
}
