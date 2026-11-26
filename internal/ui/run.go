package ui

import (
	"bufio"
	"io"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/shenri1/lazymanager/internal/manager"
)

var sudoBox = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("214")).
	Padding(1, 2).
	Width(46)

// maxRunLines caps the live output kept for a running mutation.
const maxRunLines = 2000

func stripANSI(s string) string { return manager.Sanitize(s) }

// appendOutput adds one stream segment to the visible output. 'overwrite' is
// used for progress bars: the segment replaces the last line instead of
// starting a new one. The result never exceeds maxRunLines lines.
func appendOutput(lines []string, chunk string, overwrite bool) []string {
	chunk = stripANSI(chunk)
	if overwrite && len(lines) > 0 {
		lines[len(lines)-1] = chunk
	} else {
		lines = append(lines, chunk)
	}
	if len(lines) > maxRunLines {
		lines = lines[len(lines)-maxRunLines:]
	}
	return lines
}

type runMsg struct {
	text      string
	overwrite bool
	done      bool
	err       error
}

// step is one command in a mutation. A mutation normally has a single step;
// A runs one step per manager.
type step struct {
	mgr manager.Manager
	cmd *exec.Cmd
}

type sudoCachedMsg struct {
	ok bool
}

type sudoAuthMsg struct {
	err error
}

// startMutation is the single entry point for install, remove, upgrade and
// upgrade-all. It refuses to start while another mutation is in flight and
// checks sudo credentials before opening the password modal.
func (m *model) startMutation(label string, steps []step) tea.Cmd {
	if m.running || m.checkingSudo {
		busy := m.runLabel
		if m.checkingSudo {
			busy = m.pendingLabel
		}
		m.status = "busy: " + busy + " still running"
		m.statusErr = true
		m.statusWarn = false
		return nil
	}
	if len(steps) == 0 {
		return nil
	}
	needsSudo := false
	for _, s := range steps {
		if manager.NeedsSudo(s.mgr) {
			needsSudo = true
			break
		}
	}
	if !needsSudo {
		return m.beginRun(label, steps)
	}
	if !manager.HaveSudo() {
		m.status = "sudo not found — run lazymanager as root"
		m.statusErr = true
		m.statusWarn = false
		return nil
	}

	m.pendingLabel = label
	m.pendingSteps = steps
	m.checkingSudo = true
	m.status = "checking sudo…"
	m.statusErr = false
	m.statusWarn = false
	return func() tea.Msg {
		return sudoCachedMsg{ok: manager.SudoCached()}
	}
}

func (m *model) clearPending() {
	m.pendingLabel = ""
	m.pendingSteps = nil
	m.checkingSudo = false
}

// beginRun starts the steps and wires their merged output into a channel that
// waitRun feeds back into the Update loop. A single step behaves exactly like
// the old one-command path; multiple steps run one after another and keep
// going when one fails.
func (m *model) beginRun(label string, steps []step) tea.Cmd {
	m.clearPending()
	m.running = true
	m.runLabel = label
	m.runSteps = steps
	m.runLines = nil
	m.runCh = nil
	m.runErr = nil
	m.runDone = false
	m.runFollow = true
	m.quitWarned = false
	m.quitWaiting = false
	m.holdRunOutput = false
	m.infoPkg = ""
	m.loadInfo = false
	m.detailsTitle = ""
	m.details.SetContent("")
	m.status = ""
	m.statusErr = false
	m.statusWarn = false

	ch := make(chan runMsg, 256)
	m.runCh = ch

	go func() {
		defer close(ch)
		var firstErr error
		for _, s := range steps {
			if len(steps) > 1 {
				ch <- runMsg{text: "── " + s.mgr.Name + " ──"}
			}
			err := streamStep(s.cmd, ch)
			if err != nil && firstErr == nil {
				firstErr = err
			}
			if err != nil {
				if _, ok := err.(*exec.ExitError); !ok {
					// Start/pipe failures have no command output, so show why.
					ch <- runMsg{text: err.Error()}
				}
			}
		}
		ch <- runMsg{done: true, err: firstErr}
	}()
	return tea.Batch(m.ensureSpin(), waitRun(ch))
}

// streamStep runs one command, merging its output into ch, and returns its
// exit status.
func streamStep(c *exec.Cmd, ch chan<- runMsg) error {
	pipe, err := c.StdoutPipe()
	if err != nil {
		return err
	}
	c.Stderr = c.Stdout
	c.Stdin = nil
	if err := c.Start(); err != nil {
		return err
	}
	readRunStream(pipe, ch)
	return c.Wait()
}

// readRunStream splits output on '\n' and '\r'. A bare '\r' marks an
// overwrite (progress bar); a CRLF pair is treated as a normal newline.
func readRunStream(r io.Reader, ch chan<- runMsg) {
	br := bufio.NewReader(r)
	var line strings.Builder
	for {
		b, err := br.ReadByte()
		if err != nil {
			if line.Len() > 0 {
				ch <- runMsg{text: line.String()}
			}
			return
		}
		switch b {
		case '\n':
			ch <- runMsg{text: line.String()}
			line.Reset()
		case '\r':
			if next, err := br.Peek(1); err == nil && next[0] == '\n' {
				_, _ = br.Discard(1)
				ch <- runMsg{text: line.String()}
			} else {
				ch <- runMsg{text: line.String(), overwrite: true}
			}
			line.Reset()
		default:
			line.WriteByte(b)
		}
	}
}

// waitRun returns the next stream segment. It is re-issued after every
// segment until the run-done message arrives.
func waitRun(ch <-chan runMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return runMsg{done: true}
		}
		return msg
	}
}

func (m *model) renderRun() {
	m.details.SetContent(strings.ReplaceAll(strings.Join(m.runLines, "\n"), "\t", "    "))
	if m.runFollow {
		m.details.GotoBottom()
	}
}

func (m *model) finishRun(err error) tea.Cmd {
	m.running = false
	m.runCh = nil
	m.runErr = err
	m.runDone = true
	if err != nil && len(m.runLines) == 0 {
		m.runLines = []string{err.Error()}
		m.renderRun()
	}
	if err != nil {
		m.status = m.runLabel + ": failed"
		for _, l := range m.runLines {
			if strings.Contains(l, "a password is required") {
				m.status += " — sudo asks for a password every time (timestamp_timeout=0 in sudoers?)"
				break
			}
		}
		m.statusErr = true
	} else {
		m.status = m.runLabel + ": done"
		m.statusErr = false
	}
	m.statusWarn = false
	m.spinning = m.loadPkgs || m.loadInfo || m.loadUpdates
	if m.quitWaiting {
		return tea.Quit // user asked to quit; the command finished cleanly first
	}
	return m.refreshRunManager()
}

// refreshRunManager invalidates the update cache of every manager that ran.
// The current manager, when it was one of them, reloads packages and updates;
// the others only have their update counts fetched again for the badges.
func (m *model) refreshRunManager() tea.Cmd {
	if len(m.runSteps) == 0 {
		return nil
	}
	cur, hasCur := m.currentManager()
	seen := make(map[string]bool, len(m.runSteps))
	currentRan := false
	var cmds []tea.Cmd
	for _, s := range m.runSteps {
		mgr := s.mgr
		if seen[mgr.Name] {
			continue
		}
		seen[mgr.Name] = true
		delete(m.updatesCache, mgr.Name)
		delete(m.updatesFailed, mgr.Name)
		if hasCur && cur.Name == mgr.Name {
			currentRan = true
			continue
		}
		if len(mgr.Outdated) > 0 {
			cmds = append(cmds, updatesCmd(mgr))
		}
	}
	if currentRan {
		m.holdRunOutput = true
		cmds = append(cmds, m.startPackages(true), m.startUpdates(true))
	}
	return tea.Batch(cmds...)
}

func exitCode(err error) int {
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return 1
}

// sudoErrText maps a SudoAuth error to modal text. retry is true only for a
// wrong password; anything else (not in sudoers, sudo missing, ...) cannot be
// fixed by retyping, so the caller should close the modal and show the reason.
func sudoErrText(err error) (text string, retry bool) {
	s := strings.ToLower(err.Error())
	for _, k := range []string{"incorrect password", "try again", "no password was provided"} {
		if strings.Contains(s, k) {
			return "wrong password, try again", true
		}
	}
	return firstLine(strings.TrimPrefix(err.Error(), "sudo: ")), false
}

// openSudo shows the password modal for the pending mutation.
func (m *model) openSudo() tea.Cmd {
	m.sudoOpen = true
	m.sudoChecking = false
	m.sudoErr = ""
	m.sudoAttempts = 0
	m.sudoInput.SetValue("")
	m.status = ""
	m.statusErr = false
	m.statusWarn = false
	return m.sudoInput.Focus()
}

func (m *model) closeSudo() {
	m.sudoOpen = false
	m.sudoChecking = false
	m.sudoErr = ""
	m.sudoAttempts = 0
	m.sudoInput.SetValue("")
	m.sudoInput.Blur()
	m.clearPending()
}

// submitSudo copies the password into a command closure and clears the input
// immediately, so the model never keeps it.
func (m model) submitSudo() (tea.Model, tea.Cmd) {
	if m.sudoChecking {
		return m, nil
	}
	password := m.sudoInput.Value()
	m.sudoInput.SetValue("")
	m.sudoChecking = true
	m.sudoErr = ""
	return m, func() tea.Msg {
		return sudoAuthMsg{err: manager.SudoAuth(password)}
	}
}

func (m model) sudoView() string {
	w := 50
	if m.width > 0 && m.width < w+6 {
		w = maxInt(12, m.width-6)
	}
	label := m.pendingLabel
	if label == "" {
		label = "command"
	}

	var b strings.Builder
	b.WriteString(warnTitle.Render(fit("sudo password", w-2)))
	b.WriteString("\n\n")
	b.WriteString(fit("required to: "+label, w-2))
	b.WriteString("\n\n")
	b.WriteString(m.sudoInput.View())
	b.WriteString("\n")

	switch {
	case m.sudoChecking:
		b.WriteString(dimText.Render("checking…"))
	case m.sudoErr != "":
		b.WriteString(errText.Render(fit(m.sudoErr, w-2)))
	default:
		b.WriteString(" ")
	}
	b.WriteString("\n")
	b.WriteString(dimText.Render(fit("enter confirm · esc cancel", w-2)))

	return sudoBox.Render(b.String())
}
