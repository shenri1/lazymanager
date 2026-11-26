package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/shenri1/lazymanager/internal/manager"
)

const (
	focusManagers = iota
	focusPackages
	focusUpdates
	focusDetails
)

// manyUpdates is the threshold for the yellow warning in the Updates panel.
const manyUpdates = 30

type promptKind int

const (
	promptNone promptKind = iota
	promptFilter
	promptSearch
	promptInstall
)

type confirmKind int

const (
	confirmNone confirmKind = iota
	confirmRemove
	confirmUpgrade
	confirmUpgradeAll
	confirmUpgradeEverything
)

const everythingUpToDate = "everything is up to date"

type model struct {
	cwd      string
	managers []manager.Manager
	mCursor  int

	packages []manager.Package
	filtered []manager.Package
	pCursor  int
	filter   string

	updates      []manager.Package
	updCursor    int
	updatesCache map[string][]manager.Package
	updatesErr   string
	updateWarned map[string]bool
	loadUpdates  bool

	// Managers whose last update listing failed; distinct from "still pending".
	updatesFailed map[string]string // manager -> first line of the error

	focus  int
	width  int
	height int

	details      viewport.Model
	detailsTitle string

	// A mutating command currently streaming into Details.
	running       bool
	runLabel      string
	runSteps      []step
	runLines      []string
	runCh         <-chan runMsg
	runErr        error
	runDone       bool
	runFollow     bool
	quitWarned    bool
	quitWaiting   bool // quit requested; exit once the running command ends
	holdRunOutput bool

	// A mutation waiting for sudo credentials, or the async cache check.
	pendingLabel string
	pendingSteps []step
	checkingSudo bool

	sudoOpen     bool
	sudoInput    textinput.Model
	sudoErr      string
	sudoChecking bool
	sudoAttempts int

	spin     spinner.Model
	spinning bool
	loadPkgs bool
	loadInfo bool

	input          textinput.Model
	prompt         promptKind
	confirm        confirmKind
	confirmAt      string
	confirmTargets []manager.Manager
	confirmSkipped []string // managers left out of A because their update listing failed

	searchQuery string
	infoPkg     string

	status     string
	statusErr  bool
	statusWarn bool
	showHelp   bool
}

type packagesMsg struct {
	manager string
	pkgs    []manager.Package
	err     error
}

type updatesMsg struct {
	manager string
	pkgs    []manager.Package
	err     error
}

type infoMsg struct {
	pkg  string
	text string
	err  error
}

type searchMsg struct {
	query string
	text  string
	err   error
}

// New builds the TUI model for cwd and the detected managers.
func New(cwd string, managers []manager.Manager) model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	in := textinput.New()
	in.Prompt = "> "
	in.CharLimit = 256

	sudoIn := textinput.New()
	sudoIn.Prompt = "> "
	sudoIn.Placeholder = "password"
	sudoIn.EchoMode = textinput.EchoPassword
	sudoIn.EchoCharacter = '•'
	sudoIn.CharLimit = 256
	sudoIn.Width = 42

	m := model{
		cwd:          cwd,
		managers:     managers,
		details:      viewport.New(0, 0),
		detailsTitle: "Details",
		sudoInput:    sudoIn,
		spin:         sp,
		input:        in,
		updatesCache: make(map[string][]manager.Package),
		updateWarned: make(map[string]bool),
	}
	if len(managers) == 0 {
		m.setDetails("Details", "no package managers detected")
		return m
	}
	m.loadPkgs = true
	mgr, ok := m.currentManager()
	if ok && len(mgr.Outdated) > 0 {
		m.loadUpdates = true
	}
	m.spinning = m.loadPkgs || m.loadUpdates
	return m
}

func (m model) Init() tea.Cmd {
	var cmds []tea.Cmd
	if m.spinning {
		cmds = append(cmds, m.spin.Tick)
	}
	if m.loadPkgs {
		if mgr, ok := m.currentManager(); ok {
			cmds = append(cmds, listCmd(mgr))
		}
	}
	if m.loadUpdates {
		if mgr, ok := m.currentManager(); ok {
			cmds = append(cmds, updatesCmd(mgr))
		}
	}
	// Prefetch the pending updates of every other system manager so the
	// Managers panel can show their badges and A knows what is pending. The
	// current manager is already loaded above.
	cur, hasCur := m.currentManager()
	for _, mgr := range m.managers {
		if mgr.Kind != "system" || len(mgr.Outdated) == 0 {
			continue
		}
		if hasCur && mgr.Name == cur.Name {
			continue
		}
		cmds = append(cmds, updatesCmd(mgr))
	}
	return tea.Batch(cmds...)
}

// ---- commands --------------------------------------------------------------

func listCmd(mgr manager.Manager) tea.Cmd {
	return func() tea.Msg {
		pkgs, err := mgr.Installed()
		return packagesMsg{manager: mgr.Name, pkgs: pkgs, err: err}
	}
}

func updatesCmd(mgr manager.Manager) tea.Cmd {
	return func() tea.Msg {
		pkgs, err := mgr.Updates()
		return updatesMsg{manager: mgr.Name, pkgs: pkgs, err: err}
	}
}

func infoCmd(mgr manager.Manager, pkg string) tea.Cmd {
	if len(mgr.Info) == 0 {
		return func() tea.Msg {
			return infoMsg{pkg: pkg, err: fmt.Errorf("%s does not support info", mgr.Name)}
		}
	}
	return func() tea.Msg {
		out, err := manager.Run(mgr.Cmd(false, mgr.Info, pkg))
		return infoMsg{pkg: pkg, text: out, err: err}
	}
}

func searchCmd(mgr manager.Manager, query string) tea.Cmd {
	if len(mgr.Search) == 0 {
		return func() tea.Msg {
			return searchMsg{query: query, err: fmt.Errorf("%s does not support search", mgr.Name)}
		}
	}
	return func() tea.Msg {
		out, err := manager.Run(mgr.Cmd(false, mgr.Search, query))
		return searchMsg{query: query, text: out, err: err}
	}
}

// ---- filtering / helpers ---------------------------------------------------

// filterPackages returns packages whose name contains query, ignoring case.
// An empty query (after trimming) returns the input unchanged.
func filterPackages(pkgs []manager.Package, query string) []manager.Package {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return pkgs
	}
	var out []manager.Package
	for _, p := range pkgs {
		if strings.Contains(strings.ToLower(p.Name), q) {
			out = append(out, p)
		}
	}
	return out
}

// upgradeTargets selects the system managers that A should upgrade. A manager
// is a target when it supports listing updates, its cached update count is
// positive and it has an UpgradeAll action. Systems whose manager supports
// updates but is not in the cache yet are returned as pending; those whose
// listing failed are returned as skipped instead of blocking A forever.
func upgradeTargets(mgrs []manager.Manager, cache map[string][]manager.Package, failed map[string]string) (targets []manager.Manager, pending, skipped []string) {
	for _, mgr := range mgrs {
		if mgr.Kind != "system" || len(mgr.Outdated) == 0 {
			continue
		}
		if _, bad := failed[mgr.Name]; bad {
			skipped = append(skipped, mgr.Name)
			continue
		}
		pkgs, known := cache[mgr.Name]
		if !known {
			pending = append(pending, mgr.Name)
			continue
		}
		if len(pkgs) > 0 && len(mgr.UpgradeAll) > 0 {
			targets = append(targets, mgr)
		}
	}
	return targets, pending, skipped
}

// upgradeSummary renders the confirmation detail, e.g.
// "dnf (5), flatpak (1) — brew skipped (error)".
func upgradeSummary(targets []manager.Manager, cache map[string][]manager.Package, skipped []string) string {
	parts := make([]string, 0, len(targets))
	for _, mgr := range targets {
		parts = append(parts, fmt.Sprintf("%s (%d)", mgr.Name, len(cache[mgr.Name])))
	}
	s := strings.Join(parts, ", ")
	if len(skipped) > 0 {
		s += " — " + strings.Join(skipped, ", ") + " skipped (error)"
	}
	return s
}

func (m model) currentManager() (manager.Manager, bool) {
	if m.mCursor < 0 || m.mCursor >= len(m.managers) {
		return manager.Manager{}, false
	}
	return m.managers[m.mCursor], true
}

func (m model) currentPackage() (manager.Package, bool) {
	if m.pCursor < 0 || m.pCursor >= len(m.filtered) {
		return manager.Package{}, false
	}
	return m.filtered[m.pCursor], true
}

func (m model) currentUpdate() (manager.Package, bool) {
	if m.updCursor < 0 || m.updCursor >= len(m.updates) {
		return manager.Package{}, false
	}
	return m.updates[m.updCursor], true
}

func (m *model) applyFilter() {
	m.filtered = filterPackages(m.packages, m.filter)
	if m.pCursor >= len(m.filtered) {
		m.pCursor = len(m.filtered) - 1
	}
	if m.pCursor < 0 {
		m.pCursor = 0
	}
}

func (m *model) setDetails(title, text string) {
	if title == "" {
		title = "Details"
	}
	m.runDone = false
	m.detailsTitle = title
	m.details.SetContent(strings.ReplaceAll(stripANSI(text), "\t", "    "))
	m.details.GotoTop()
}

func (m *model) ensureSpin() tea.Cmd {
	if m.spinning {
		return nil
	}
	m.spinning = true
	return m.spin.Tick
}

// startPackages reloads the package list of the current manager. When quiet is
// false the status bar announces the refresh.
func (m *model) startPackages(quiet bool) tea.Cmd {
	mgr, ok := m.currentManager()
	if !ok {
		return nil
	}
	m.packages = nil
	m.filtered = nil
	m.pCursor = 0
	m.loadPkgs = true
	if !quiet {
		m.status = "refreshing packages…"
		m.statusErr = false
		m.statusWarn = false
	}
	return tea.Batch(m.ensureSpin(), listCmd(mgr))
}

// startUpdates loads the current manager's updates. With force the cache entry
// is discarded; otherwise a cached list is used immediately.
func (m *model) startUpdates(force bool) tea.Cmd {
	mgr, ok := m.currentManager()
	if !ok {
		return nil
	}
	m.updatesErr = ""
	if len(mgr.Outdated) == 0 {
		m.updates = nil
		m.updCursor = 0
		m.loadUpdates = false
		return nil
	}
	if force {
		delete(m.updatesCache, mgr.Name)
		delete(m.updatesFailed, mgr.Name)
	} else if cached, ok := m.updatesCache[mgr.Name]; ok {
		m.updates = cached
		m.updCursor = 0
		m.loadUpdates = false
		return nil
	}
	delete(m.updatesFailed, mgr.Name) // retrying: back to "pending"
	m.updates = nil
	m.updCursor = 0
	m.loadUpdates = true
	return tea.Batch(m.ensureSpin(), updatesCmd(mgr))
}

// managerChanged resets the package list and loads packages and updates for
// the new manager.
func (m *model) managerChanged() tea.Cmd {
	m.infoPkg = ""
	m.loadInfo = false
	m.holdRunOutput = false
	if m.filter != "" {
		m.filter = ""
		m.input.SetValue("")
	}
	if !m.running {
		m.setDetails("Details", "")
	}
	pkgs := m.startPackages(false)
	updates := m.startUpdates(false)
	return tea.Batch(pkgs, updates)
}

// packageChanged loads information for the newly selected package. While a
// mutation is running the Details panel keeps the live output.
func (m *model) packageChanged() tea.Cmd {
	pkg, ok := m.currentPackage()
	if !ok {
		return nil
	}
	if m.running {
		m.status = "running…"
		m.statusErr = false
		m.statusWarn = false
		return nil
	}
	m.holdRunOutput = false
	m.infoPkg = pkg.Name
	m.loadInfo = true
	m.setDetails("Details", "loading info…")
	return tea.Batch(m.ensureSpin(), infoCmd(m.managers[m.mCursor], pkg.Name))
}

// updateInfo loads information for the selected update. While a mutation is
// running the Details panel keeps the live output.
func (m *model) updateInfo() tea.Cmd {
	if m.running {
		m.status = "running…"
		m.statusErr = false
		m.statusWarn = false
		return nil
	}
	pkg, ok := m.currentUpdate()
	if !ok {
		m.status = "no update selected"
		m.statusErr = true
		m.statusWarn = false
		return nil
	}
	mgr, ok := m.currentManager()
	if !ok {
		return nil
	}
	m.holdRunOutput = false
	m.infoPkg = pkg.Name
	m.loadInfo = true
	m.setDetails("Details: "+pkg.Name, "loading info…")
	return tea.Batch(m.ensureSpin(), infoCmd(mgr, pkg.Name))
}

// infoIfChanged loads info when filtering moved the selection to another
// package. old is the name selected before the filter was applied.
func (m *model) infoIfChanged(old string) tea.Cmd {
	pkg, ok := m.currentPackage()
	if !ok || pkg.Name == old {
		return nil
	}
	return m.packageChanged()
}

// firstLine returns the first non-empty line of s, or "error" when there is none.
func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return "error"
}

// failureReport lists why each skipped manager's update check failed.
func failureReport(skipped []string, failed map[string]string) string {
	lines := make([]string, 0, len(skipped))
	for _, name := range skipped {
		lines = append(lines, name+": "+failed[name])
	}
	return strings.Join(lines, "\n")
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
