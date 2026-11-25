package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"lazymanager/internal/manager"
)

// ---- styles ----------------------------------------------------------------

var (
	focusedPanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("42"))
	dimPanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240"))
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
	warnTitle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	selectedItem = lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(lipgloss.Color("255"))
	dimText      = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	errText      = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	statusText   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	warnText     = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	allGoodText  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	okTitle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	errTitle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
)

// ---- layout ----------------------------------------------------------------

type layout struct {
	leftW, rightW    int
	mgrH, pkgH, updH int
	bodyH            int
}

func (m model) layout() layout {
	leftW := m.width * 35 / 100
	if leftW < 24 {
		leftW = 24
	}
	if leftW > m.width-24 {
		leftW = m.width / 2
	}
	if leftW < 10 {
		leftW = 10
	}
	if leftW >= m.width {
		leftW = m.width - 1
	}
	rightW := m.width - leftW

	bodyH := m.height - 1
	if bodyH < 12 {
		bodyH = 12
	}
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
	m.input.Width = clamp(m.width-42, 6, 40)
	m.sudoInput.Width = clamp(m.width-18, 10, 42)
}

// ---- view ------------------------------------------------------------------

func (m model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return "starting lazymanager…"
	}
	if m.showHelp {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, helpView())
	}
	if m.sudoOpen {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.sudoView())
	}

	l := m.layout()
	innerLeft := l.leftW - 2
	if innerLeft < 1 {
		innerLeft = 1
	}

	mgrBody := m.managerLines(innerLeft, l.mgrH-3)
	mgrPanel := renderPanel("Managers", mgrBody, l.leftW, l.mgrH, m.focus == focusManagers, false, "")

	pkgTitle := "Packages"
	if m.filter != "" {
		pkgTitle = "Packages /" + m.filter
	}
	pkgBody := m.packageLines(innerLeft, l.pkgH-3)
	pkgPanel := renderPanel(pkgTitle, pkgBody, l.leftW, l.pkgH, m.focus == focusPackages, m.loadPkgs, m.spin.View())

	updTitle, updStyle := m.updatesTitle()
	updBody := m.updateLines(innerLeft, l.updH-3)
	updPanel := renderPanelStyle(updTitle, updBody, l.leftW, l.updH, m.focus == focusUpdates, m.loadUpdates, m.spin.View(), updStyle)

	left := lipgloss.JoinVertical(lipgloss.Left, mgrPanel, pkgPanel, updPanel)

	detailBody := m.detailLines(maxInt(1, l.rightW-2))
	detailsTitle := m.detailsTitle
	detailsStyle := titleStyle
	loading := m.loadInfo
	if m.running {
		detailsTitle = m.spin.View() + " " + m.runLabel
		loading = false
	} else if m.runDone {
		if m.runErr != nil {
			detailsTitle = fmt.Sprintf("✗ %s (exit %d)", m.runLabel, exitCode(m.runErr))
			detailsStyle = errTitle
		} else {
			detailsTitle = "✓ " + m.runLabel
			detailsStyle = okTitle
		}
	}
	detailsPanel := renderPanelStyle(detailsTitle, detailBody, l.rightW, l.bodyH, m.focus == focusDetails, loading, m.spin.View(), detailsStyle)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, detailsPanel)

	return lipgloss.JoinVertical(lipgloss.Left, body, m.statusBar())
}

func (m model) managerLines(w, visible int) []string {
	if len(m.managers) == 0 {
		return []string{dimText.Render(fit("no package managers detected", w))}
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
// "dnf  5↑". The badge is green, or yellow at the manyUpdates threshold.
func (m model) managerLine(mgr manager.Manager, selected bool, w int) string {
	text := mgr.Name
	if mgr.Kind == "project" {
		text += " (project)"
	}
	pkgs, known := m.updatesCache[mgr.Name]
	n := len(pkgs)
	if !known || n == 0 {
		return listItem(text, selected, w)
	}

	suffix := fmt.Sprintf("%d↑", n)
	plain := text + "  " + suffix
	avail := maxInt(0, w-2)
	if len([]rune(plain)) > avail {
		return listItem(fit(plain, avail), selected, w)
	}

	countStyle := allGoodText
	if n >= manyUpdates {
		countStyle = warnText
	}
	if selected {
		countStyle = countStyle.Background(lipgloss.Color("237"))
		base := "▸ " + text + "  "
		pad := ""
		if used := len([]rune(base)) + len([]rune(suffix)); used < w {
			pad = strings.Repeat(" ", w-used)
		}
		return selectedItem.Render(base) + countStyle.Render(suffix) + selectedItem.Render(pad)
	}
	return "  " + text + "  " + countStyle.Render(suffix)
}

func (m model) packageLines(w, visible int) []string {
	if len(m.filtered) == 0 {
		msg := "no packages"
		if len(m.packages) > 0 {
			msg = "no matching packages"
		}
		return []string{dimText.Render(fit(msg, w))}
	}
	if visible < 1 {
		return nil
	}
	start := scrollStart(m.pCursor, len(m.filtered), visible)
	end := minInt(start+visible, len(m.filtered))
	var lines []string
	for i := start; i < end; i++ {
		pkg := m.filtered[i]
		text := pkg.Name
		if pkg.Version != "" {
			text += "  " + pkg.Version
		}
		lines = append(lines, listItem(text, i == m.pCursor, w))
	}
	return lines
}

func (m model) updatesTitle() (string, lipgloss.Style) {
	mgr, ok := m.currentManager()
	if !ok || len(mgr.Outdated) == 0 {
		return "Updates", titleStyle
	}
	if m.loadUpdates || m.updatesErr != "" {
		return "Updates", titleStyle
	}
	n := len(m.updates)
	title := fmt.Sprintf("Updates (%d)", n)
	if n >= manyUpdates {
		title += " ⚠"
		return title, warnTitle
	}
	return title, titleStyle
}

func (m model) updateLines(w, visible int) []string {
	mgr, ok := m.currentManager()
	if !ok {
		return []string{dimText.Render(fit("no manager selected", w))}
	}
	if len(mgr.Outdated) == 0 {
		return []string{dimText.Render(fit("updates not supported for "+mgr.Name, w))}
	}
	if m.updatesErr != "" {
		return []string{errText.Render(fit(m.updatesErr, w))}
	}
	if m.loadUpdates && len(m.updates) == 0 {
		return []string{dimText.Render(fit("loading updates…", w))}
	}
	if len(m.updates) == 0 {
		return allGoodLines(w, visible)
	}
	if visible < 1 {
		return nil
	}
	start := scrollStart(m.updCursor, len(m.updates), visible)
	end := minInt(start+visible, len(m.updates))
	var lines []string
	for i := start; i < end; i++ {
		pkg := m.updates[i]
		text := pkg.Name
		if pkg.Version != "" {
			text += "  " + pkg.Version
		}
		lines = append(lines, listItem(text, i == m.updCursor, w))
	}
	return lines
}

// allGoodLines renders a small green "all good" picture, centered.
func allGoodLines(w, visible int) []string {
	art := []string{fit("✓", w), fit("up to date", w)}
	top := 0
	if visible >= len(art)+2 {
		top = (visible - len(art)) / 2
	}
	center := allGoodText.Width(w).Align(lipgloss.Center)
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

func (m model) detailLines(w int) []string {
	raw := strings.Split(m.details.View(), "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		if !strings.Contains(line, "\x1b") {
			line = fit(line, w)
		}
		lines = append(lines, line)
	}
	return lines
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
		switch m.confirm {
		case confirmRemove:
			return fit("Remove "+m.confirmAt+"? (y/n)", m.width)
		case confirmUpgrade:
			return fit("Upgrade "+m.confirmAt+"? (y/n)", m.width)
		case confirmUpgradeAll:
			return fit("Upgrade all packages? (y/n)", m.width)
		case confirmUpgradeEverything:
			return fit("Upgrade everything? "+upgradeSummary(m.confirmTargets, m.updatesCache)+" (y/n)", m.width)
		}
	}
	if m.status != "" {
		if m.statusErr {
			return errText.Render(fit(m.status, m.width))
		}
		if m.statusWarn {
			return warnText.Render(fit(m.status, m.width))
		}
		if m.status == everythingUpToDate {
			return allGoodText.Render(fit(m.status, m.width))
		}
		return statusText.Render(fit(m.status, m.width))
	}
	var hints string
	switch m.focus {
	case focusManagers:
		hints = "j/k move · s search · i install · A upgrade everything · r refresh · tab focus · ? help · q quit"
	case focusPackages:
		hints = "j/k move · / filter · enter info · i install · d remove · u upgrade · U upgrade-all"
	case focusUpdates:
		hints = "j/k move · enter info · u upgrade · U upgrade-all · A upgrade everything · r refresh · tab focus"
	case focusDetails:
		hints = "j/k scroll · pgup/pgdn page · g/G top/bottom · tab focus · ? help · q quit"
	}
	return statusText.Render(fit(hints, m.width))
}

func helpView() string {
	body := `lazymanager — keys

  tab / shift+tab    switch panel
  ← → / h l          previous / next panel
  1 2 3 4            managers / packages / updates / details
  j k ↑ ↓            move
  pgup pgdown        move a page / scroll details
  g G                top / bottom
  /                  filter packages
  s                  search remote
  i                  install
  d                  remove selected package
  u                  upgrade selected package or update
  U                  upgrade all
  A                  upgrade everything (all system managers with updates)
  r                  refresh packages and updates
  enter              load info
  ?                  close help
  q / ctrl+c         quit

  mouse              click to focus/select, wheel to move/scroll
  esc                cancel prompt / clear filter`
	return dimPanel.Padding(1, 2).Render(body)
}

// ---- rendering helpers -----------------------------------------------------

func renderPanel(title string, body []string, w, h int, focused, loading bool, spin string) string {
	return renderPanelStyle(title, body, w, h, focused, loading, spin, titleStyle)
}

func renderPanelStyle(title string, body []string, w, h int, focused, loading bool, spin string, tstyle lipgloss.Style) string {
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
	style := dimPanel
	if focused {
		style = focusedPanel
	}
	return style.Width(innerW).Height(innerH).Render(strings.Join(lines, "\n"))
}

func listItem(text string, selected bool, w int) string {
	if selected {
		return selectedItem.Render(padRight(fit("▸ "+text, w), w))
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

// fit shortens s to at most w runes, adding an ellipsis when it does not fit.
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w == 1 {
		return string(r[:1])
	}
	return string(r[:w-1]) + "…"
}

func padRight(s string, w int) string {
	n := len([]rune(s))
	if n >= w {
		return s
	}
	return s + strings.Repeat(" ", w-n)
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
