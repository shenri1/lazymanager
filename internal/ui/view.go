package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/shenri1/lazymanager/internal/manager"
)

// ---- layout ----------------------------------------------------------------

type layout struct {
	leftW, rightW    int
	mgrH, pkgH, updH int
	bodyH            int
}

// minWidth and minHeight are the smallest terminal the panel layout supports;
// below it View shows a notice instead of overlapping, clipped panels.
const minWidth, minHeight = 60, 15

func (m model) tooSmall() bool { return m.width < minWidth || m.height < minHeight }

func (m model) layout() layout {
	leftW := maxInt(24, m.width*35/100)
	rightW := m.width - leftW
	bodyH := m.height - 1
	mgrH := bodyH * 25 / 100
	if mgrH < 4 {
		mgrH = 4
	}
	pkgH := bodyH * 45 / 100
	if pkgH < 3 {
		pkgH = 3
	}
	updH := bodyH - mgrH - pkgH
	if updH < 5 {
		need := 5 - updH
		take := minInt(need, pkgH-3)
		pkgH -= take
		need -= take
		if need > 0 {
			take = minInt(need, mgrH-4)
			mgrH -= take
		}
		updH = bodyH - mgrH - pkgH
		if updH < 1 {
			updH = 1
		}
	}
	return layout{leftW: leftW, rightW: rightW, mgrH: mgrH, pkgH: pkgH, updH: updH, bodyH: bodyH}
}

func (m *model) resize() {
	l := m.layout()
	m.details.Width = maxInt(1, l.rightW-2)
	m.details.Height = maxInt(1, l.bodyH-3)
	if m.running || m.runDone {
		m.renderRun()
	} else {
		m.refreshDetails()
	}
	m.input.Width = clamp(m.width-42, 6, 40)
	m.sudoInput.Width = clamp(m.width-18, 10, 42)
}

// ---- view ------------------------------------------------------------------

func (m model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return "starting lazymanager…"
	}
	if m.tooSmall() {
		msg := fmt.Sprintf("terminal too small\nneed %d×%d, have %d×%d", minWidth, minHeight, m.width, m.height)
		lines := strings.Split(msg, "\n")
		for i := range lines {
			lines[i] = fit(lines[i], m.width)
		}
		return m.styles.paintScreen(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.styles.warn.Render(strings.Join(lines, "\n"))))
	}
	if m.showLog {
		return m.styles.paintScreen(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.logView()))
	}
	if m.showHelp {
		return m.styles.paintScreen(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.helpView()))
	}
	if m.sudoOpen {
		return m.styles.paintScreen(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.sudoView()))
	}

	l := m.layout()
	innerLeft := l.leftW - 2
	if innerLeft < 1 {
		innerLeft = 1
	}

	mgrBody := m.managerLines(innerLeft, l.mgrH-3)
	mgrTitle := m.styles.titleWith(m.styles.icons.managers, "Managers")
	mgrPanel := m.renderPanel(mgrTitle, mgrBody, l.leftW, l.mgrH, m.focus == focusManagers, false, "", m.styles.title)

	pkgTitle := "Packages"
	if m.filter != "" {
		pkgTitle = "Packages /" + m.filter
	}
	pkgTitle = m.styles.titleWith(m.styles.icons.packages, pkgTitle)
	pkgBody := m.packageLines(innerLeft, l.pkgH-3)
	pkgPanel := m.renderPanel(pkgTitle, pkgBody, l.leftW, l.pkgH, m.focus == focusPackages, m.loadPkgs, m.spin.View(), m.styles.title)

	updTitle, updStyle := m.updatesTitle()
	updTitle = m.styles.titleWith(m.styles.icons.updates, updTitle)
	updBody := m.updateLines(innerLeft, l.updH-3)
	updPanel := m.renderPanel(updTitle, updBody, l.leftW, l.updH, m.focus == focusUpdates, m.loadUpdates, m.spin.View(), updStyle)

	left := lipgloss.JoinVertical(lipgloss.Left, mgrPanel, pkgPanel, updPanel)

	detailBody := m.detailLines()
	detailsTitle := m.styles.titleWith(m.styles.icons.details, m.detailsTitle)
	detailsStyle := m.styles.title
	loading := m.loadInfo
	if m.running {
		detailsTitle = m.spin.View() + " " + m.runLabel
		loading = false
	} else if m.runDone {
		if m.runErr != nil {
			detailsTitle = fmt.Sprintf("%s %s (exit %d)", m.styles.icons.error, m.runLabel, exitCode(m.runErr))
			detailsStyle = m.styles.errTitle
		} else {
			detailsTitle = m.styles.icons.check + " " + m.runLabel
			detailsStyle = m.styles.okTitle
		}
	}
	detailsPanel := m.renderPanel(detailsTitle, detailBody, l.rightW, l.bodyH, m.focus == focusDetails, loading, m.spin.View(), detailsStyle)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, detailsPanel)

	return m.styles.paintScreen(lipgloss.JoinVertical(lipgloss.Left, body, m.statusBar()))
}

func (m model) managerLines(w, visible int) []string {
	if len(m.managers) == 0 {
		return []string{m.styles.dim.Render(fit("no package managers detected", w))}
	}
	if visible < 1 {
		return nil
	}
	start := scrollStart(m.mCursor, len(m.managers), visible)
	end := minInt(start+visible, len(m.managers))
	var lines []string
	for i := start; i < end; i++ {
		lines = append(lines, m.managerLine(m.managers[i], i == m.mCursor, w))
	}
	return lines
}

// managerLine renders one manager with its cached pending-update badge, e.g.
// "dnf  5↑". The badge is green, or yellow at the many-updates threshold.
func (m model) managerLine(mgr manager.Manager, selected bool, w int) string {
	focused := m.focus == focusManagers
	text := mgr.Name
	if mgr.Kind == "project" {
		text += " (project)"
	}
	pkgs, known := m.updatesCache[mgr.Name]
	n := len(pkgs)
	var suffix string
	countStyle := m.styles.allGood
	switch {
	case m.updatesFailed[mgr.Name] != "":
		suffix, countStyle = m.styles.icons.failure, m.styles.err // update listing failed
	case known && n > 0:
		suffix = fmt.Sprintf("%d%s", n, m.styles.icons.updateBadge)
		if n >= m.cfg.Managers.ManyUpdatesThreshold {
			countStyle = m.styles.warn
		}
	default:
		return m.listItem(text, selected, focused, w)
	}
	plain := text + "  " + suffix
	avail := maxInt(0, w-2)
	if ansi.StringWidth(plain) > avail {
		return m.listItem(fit(plain, avail), selected, focused, w)
	}

	if selected {
		row := m.styles.icons.pointer + " " + plain
		return m.styles.selection(focused).Render(padRight(fit(row, w), w))
	}
	return "  " + text + "  " + countStyle.Render(suffix)
}

func (m model) packageLines(w, visible int) []string {
	if len(m.filtered) == 0 {
		msg := "no packages"
		if len(m.packages) > 0 {
			msg = "no matching packages"
		}
		return []string{m.styles.dim.Render(fit(msg, w))}
	}
	if visible < 1 {
		return nil
	}
	start := scrollStart(m.pCursor, len(m.filtered), visible)
	end := minInt(start+visible, len(m.filtered))
	var lines []string
	focused := m.focus == focusPackages
	for i := start; i < end; i++ {
		pkg := m.filtered[i]
		text := pkg.Name
		if pkg.Version != "" {
			text += "  " + pkg.Version
		}
		lines = append(lines, m.listItem(text, i == m.pCursor, focused, w))
	}
	return lines
}

func (m model) updatesTitle() (string, lipgloss.Style) {
	mgr, ok := m.currentManager()
	if !ok || len(mgr.Outdated) == 0 {
		return "Updates", m.styles.title
	}
	if m.loadUpdates || m.updatesErr != "" {
		return "Updates", m.styles.title
	}
	n := len(m.updates)
	title := fmt.Sprintf("Updates (%d)", n)
	if n >= m.cfg.Managers.ManyUpdatesThreshold {
		title += " " + m.styles.icons.warning
		return title, m.styles.warnTitle
	}
	return title, m.styles.title
}

func (m model) updateLines(w, visible int) []string {
	mgr, ok := m.currentManager()
	if !ok {
		return []string{m.styles.dim.Render(fit("no manager selected", w))}
	}
	if len(mgr.Outdated) == 0 {
		return []string{m.styles.dim.Render(fit("updates not supported for "+mgr.Name, w))}
	}
	if m.updatesErr != "" {
		return []string{m.styles.err.Render(fit(m.updatesErr, w))}
	}
	if m.loadUpdates && len(m.updates) == 0 {
		return []string{m.styles.dim.Render(fit("loading updates…", w))}
	}
	if len(m.updates) == 0 {
		return m.allGoodLines(w, visible)
	}
	if visible < 1 {
		return nil
	}
	start := scrollStart(m.updCursor, len(m.updates), visible)
	end := minInt(start+visible, len(m.updates))
	focused := m.focus == focusUpdates
	var lines []string
	for i := start; i < end; i++ {
		pkg := m.updates[i]
		text := pkg.Name
		if pkg.Version != "" {
			text += "  " + pkg.Version
		}
		lines = append(lines, m.listItem(text, i == m.updCursor, focused, w))
	}
	return lines
}

// allGoodLines renders a small "all good" picture, centered.
func (m model) allGoodLines(w, visible int) []string {
	art := []string{fit(m.styles.icons.check, w), fit("up to date", w)}
	top := 0
	if visible >= len(art)+2 {
		top = (visible - len(art)) / 2
	}
	center := m.styles.allGood.Width(w).Align(lipgloss.Center)
	lines := make([]string, 0, top+len(art))
	for i := 0; i < top; i++ {
		lines = append(lines, "")
	}
	for _, line := range art {
		lines = append(lines, center.Render(line))
	}
	if len(lines) > visible && visible >= 0 {
		lines = lines[:visible]
	}
	return lines
}

func (m model) detailLines() []string {
	// Content is wrapped to the viewport width before SetContent, so the
	// viewport already returns complete lines without truncation.
	return strings.Split(m.details.View(), "\n")
}

func (m model) statusBar() string {
	if m.prompt != promptNone {
		var prefix string
		switch m.prompt {
		case promptFilter:
			prefix = "filter packages: "
		case promptSearch:
			if mgr, ok := m.currentManager(); ok {
				prefix = "search " + mgr.Name + ": "
			} else {
				prefix = "search: "
			}
		case promptInstall:
			if mgr, ok := m.currentManager(); ok {
				prefix = "install with " + mgr.Name + ": "
			} else {
				prefix = "install: "
			}
		}
		return fit(prefix+m.input.View()+"  (enter/esc)", m.width)
	}
	if m.confirm != confirmNone {
		var text string
		switch m.confirm {
		case confirmRemove:
			text = "Remove " + m.confirmAt + "?"
		case confirmUpgrade:
			text = "Upgrade " + m.confirmAt + "?"
		case confirmUpgradeAll:
			text = "Upgrade all packages?"
		case confirmUpgradeEverything:
			text = "Upgrade everything? " + upgradeSummary(m.confirmTargets, m.updatesCache, m.confirmSkipped)
		}
		if cmd := m.confirmCommand(); cmd != "" {
			text += " $ " + cmd
		}
		return fitSuffix(text, " (y/n)", m.width)
	}
	if m.status != "" {
		if m.statusErr {
			return m.styles.err.Render(fit(m.styles.icons.error+" "+m.status, m.width))
		}
		if m.statusWarn {
			return m.styles.warn.Render(fit(m.styles.icons.warning+" "+m.status, m.width))
		}
		if m.status == everythingUpToDate {
			return m.styles.allGood.Render(fit(m.styles.icons.check+" "+m.status, m.width))
		}
		return m.styles.status.Render(fit(m.status, m.width))
	}
	k := m.keys
	var hints string
	switch m.focus {
	case focusManagers:
		hints = fmt.Sprintf("%s move · %s search · %s install · %s upgrade everything · %s refresh · %s focus · %s help · %s quit",
			k.hint(actUp, actDown), k.label(actSearch), k.label(actInstall), k.label(actUpgradeEverything),
			k.label(actRefresh), k.label(actNextPanel), k.label(actHelp), k.label(actQuit))
	case focusPackages:
		hints = fmt.Sprintf("%s move · %s filter · %s info · %s install · %s remove · %s upgrade · %s upgrade-all",
			k.hint(actUp, actDown), k.label(actFilter), k.label(actInfo), k.label(actInstall),
			k.label(actRemove), k.label(actUpgrade), k.label(actUpgradeAll))
	case focusUpdates:
		hints = fmt.Sprintf("%s move · %s info · %s upgrade · %s upgrade-all · %s upgrade everything · %s refresh · %s focus",
			k.hint(actUp, actDown), k.label(actInfo), k.label(actUpgrade), k.label(actUpgradeAll),
			k.label(actUpgradeEverything), k.label(actRefresh), k.label(actNextPanel))
	case focusDetails:
		hints = fmt.Sprintf("%s scroll · %s page · %s/%s top/bottom · %s focus · %s help · %s quit",
			k.hint(actUp, actDown), k.hint(actPageUp, actPageDown), k.label(actTop), k.label(actBottom),
			k.label(actNextPanel), k.label(actHelp), k.label(actQuit))
	}
	return m.styles.status.Render(fit(hints, m.width))
}

// confirmCommand renders the exact command that will run if the user confirms.
func (m model) confirmCommand() string {
	switch m.confirm {
	case confirmRemove:
		if mgr, ok := m.currentManager(); ok {
			return commandLine(mgr.Cmd(true, mgr.Remove, m.confirmAt))
		}
	case confirmUpgrade:
		if mgr, ok := m.currentManager(); ok {
			return commandLine(mgr.Cmd(true, mgr.Upgrade, m.confirmAt))
		}
	case confirmUpgradeAll:
		if mgr, ok := m.currentManager(); ok {
			return commandLine(mgr.Cmd(true, mgr.UpgradeAll))
		}
	case confirmUpgradeEverything:
		var first string
		n := 0
		for _, mgr := range m.confirmTargets {
			if cmd := commandLine(mgr.Cmd(true, mgr.UpgradeAll)); cmd != "" {
				if first == "" {
					first = cmd
				}
				n++
			}
		}
		if n > 1 {
			return fmt.Sprintf("%s (+%d more)", first, n-1)
		}
		return first
	}
	return ""
}

// helpWindow returns how many body lines (everything under the title) fit and
// the clamped scroll offset into them.
func (m model) helpWindow() (visible, offset int) {
	body := strings.Count(helpText(m.keys), "\n") // lines after the title
	visible = minInt(body, m.height-3)            // border top/bottom + title
	offset = clamp(m.helpScroll, 0, maxInt(0, body-visible))
	return visible, offset
}

// helpView fits the key list to the terminal; when it is taller than the
// screen the title stays and the body becomes a window that j/k scroll.
func (m model) helpView() string {
	lines := strings.Split(helpText(m.keys), "\n")
	title, body := lines[0], lines[1:]
	visible, offset := m.helpWindow()
	if visible < len(body) {
		hint := m.keys.hint(actUp, actDown)
		if hint == "" {
			hint = "j/k"
		}
		title += " (" + hint + " scroll)"
	}
	lines = append([]string{title}, body[offset:offset+visible]...)
	w := minInt(76, m.width-4) // border + 1 col padding each side
	for i := range lines {
		lines[i] = fit(lines[i], w)
	}
	return m.styles.panelDim.Padding(0, 1).Render(strings.Join(lines, "\n"))
}

// logBodyLen is the number of body lines (header plus entries) in the log.
func (m model) logBodyLen() int {
	if len(m.cmdLog) == 0 {
		return 3 // header + blank + "no commands"
	}
	return 1 + len(m.cmdLog)
}

func (m model) logVisible() int {
	return minInt(m.logBodyLen(), maxInt(1, m.height-3))
}

func (m model) logMaxScroll() int {
	return maxInt(0, m.logBodyLen()-m.logVisible())
}

func (m model) logPageSize() int {
	return maxInt(1, m.logVisible()-1)
}

// logView renders the session command history: time, exit code, duration and
// the exact command line.
func (m model) logView() string {
	w := minInt(110, m.width-4)
	if w < 10 {
		w = maxInt(1, m.width-4)
	}
	body := []string{fmt.Sprintf("%-8s  %-4s  %-9s  %s", "TIME", "EXIT", "DURATION", "COMMAND")}
	if len(m.cmdLog) == 0 {
		body = append(body, "", "no commands in this session yet")
	} else {
		for _, e := range m.cmdLog {
			body = append(body, fmt.Sprintf("%-8s  %-4d  %-9s  %s",
				e.Time.Format("15:04:05"), e.ExitCode,
				e.Duration.Round(time.Millisecond).String(), e.Command))
		}
	}
	visible := minInt(len(body), maxInt(1, m.height-3))
	offset := clamp(m.logScroll, 0, maxInt(0, len(body)-visible))

	title := fmt.Sprintf("Command Log (%d)", len(m.cmdLog))
	closeKey := m.keys.label(actCommandLog)
	if visible < len(body) {
		scroll := m.keys.hint(actUp, actDown)
		if scroll == "" {
			scroll = "j/k"
		}
		title += " (" + scroll + " scroll"
		if closeKey != "" {
			title += " · " + closeKey + " close"
		}
		title += ")"
	} else if closeKey != "" {
		title += " (" + closeKey + " close)"
	}

	lines := append([]string{title}, body[offset:offset+visible]...)
	for i := range lines {
		lines[i] = fit(lines[i], w)
	}
	return m.styles.panelDim.Padding(0, 1).Render(strings.Join(lines, "\n"))
}

// ---- rendering helpers -----------------------------------------------------

func (m model) renderPanel(title string, body []string, w, h int, focused, loading bool, spin string, tstyle lipgloss.Style) string {
	innerW := w - 2
	innerH := h - 2
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}
	if loading && spin != "" {
		title = spin + " " + title
	}
	lines := make([]string, 0, innerH)
	lines = append(lines, tstyle.Render(fit(title, innerW)))
	for i := 0; i < innerH-1 && i < len(body); i++ {
		lines = append(lines, body[i])
	}
	for len(lines) < innerH {
		lines = append(lines, "")
	}
	return m.styles.panel(focused).Width(innerW).Height(innerH).Render(strings.Join(lines, "\n"))
}

func (m model) listItem(text string, selected, focused bool, w int) string {
	if selected {
		row := m.styles.icons.pointer + " " + text
		return m.styles.selection(focused).Render(padRight(fit(row, w), w))
	}
	return "  " + fit(text, maxInt(0, w-2))
}

func scrollStart(cursor, total, visible int) int {
	if visible < 1 || total <= visible {
		return 0
	}
	start := 0
	if cursor >= visible {
		start = cursor - visible + 1
	}
	if start > total-visible {
		start = total - visible
	}
	if start < 0 {
		start = 0
	}
	return start
}

// fit shortens s to at most w terminal columns (wide CJK/emoji count as 2,
// ANSI styling is preserved), adding an ellipsis when it does not fit.
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= w {
		return s
	}
	return ansi.Truncate(s, w, "…")
}

func padRight(s string, w int) string {
	n := ansi.StringWidth(s)
	if n >= w {
		return s
	}
	return s + strings.Repeat(" ", w-n)
}

// fitSuffix truncates s but always keeps suffix visible, so prompts such as
// "(y/n)" are never cut off.
func fitSuffix(s, suffix string, w int) string {
	if ansi.StringWidth(s)+ansi.StringWidth(suffix) <= w {
		return s + suffix
	}
	return ansi.Truncate(s, maxInt(0, w-ansi.StringWidth(suffix)), "…") + suffix
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
