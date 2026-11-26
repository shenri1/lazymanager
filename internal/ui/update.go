package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/shenri1/lazymanager/internal/manager"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		if m.loadPkgs || m.loadInfo || m.loadUpdates || m.running {
			return m, cmd
		}
		m.spinning = false
		return m, nil

	case packagesMsg:
		mgr, ok := m.currentManager()
		if !ok || msg.manager != mgr.Name || msg.seq != m.pkgSeq {
			return m, nil // another manager, or superseded by a newer request
		}
		m.loadPkgs = false
		hold := m.holdRunOutput
		m.holdRunOutput = false
		if msg.err != nil {
			m.packages = nil
			m.filtered = nil
			m.pCursor = 0
			m.status = msg.manager + ": " + msg.err.Error()
			m.statusErr = true
			m.statusWarn = false
			if !m.running && !hold {
				m.setDetails("Details: "+msg.manager, msg.err.Error())
			}
			return m, nil
		}
		m.packages = msg.pkgs
		m.applyFilter()
		if m.status == "refreshing packages…" || m.status == "refreshing…" {
			m.status = ""
		}
		if len(m.filtered) > 0 {
			if hold {
				return m, nil
			}
			return m, m.packageChanged()
		}
		if !m.running && !hold {
			m.setDetails("Details", "no packages")
		}
		return m, nil

	case updatesMsg:
		if msg.seq != m.updSeq[msg.manager] {
			return m, nil // superseded by a newer request for this manager
		}
		if m.updatesCache == nil {
			m.updatesCache = make(map[string][]manager.Package)
		}
		if m.updateWarned == nil {
			m.updateWarned = make(map[string]bool)
		}
		if m.updatesFailed == nil {
			m.updatesFailed = make(map[string]string)
		}
		if msg.err == nil {
			m.updatesCache[msg.manager] = msg.pkgs
			delete(m.updatesFailed, msg.manager)
		} else {
			m.updatesFailed[msg.manager] = firstLine(msg.err.Error())
		}
		mgr, ok := m.currentManager()
		if !ok || msg.manager != mgr.Name {
			return m, nil
		}
		m.loadUpdates = false
		sel, idx := m.updSel, m.updIdx
		m.updSel = ""
		if msg.err != nil {
			m.updates = nil
			m.updCursor = 0
			m.updatesErr = msg.err.Error()
			m.status = msg.manager + " updates: " + msg.err.Error()
			m.statusErr = true
			m.statusWarn = false
			return m, nil
		}
		m.updates = msg.pkgs
		m.updCursor = 0
		if sel != "" && len(msg.pkgs) > 0 { // refresh: keep the selection by name, else by position
			m.updCursor = clamp(idx, 0, len(msg.pkgs)-1)
			for i, p := range msg.pkgs {
				if p.Name == sel {
					m.updCursor = i
					break
				}
			}
		}
		m.updatesErr = ""
		if len(msg.pkgs) >= m.cfg.Managers.ManyUpdatesThreshold && !m.updateWarned[msg.manager] {
			m.updateWarned[msg.manager] = true
			m.status = fmt.Sprintf("%s: %d updates pending — press %s to upgrade all", msg.manager, len(msg.pkgs), m.keys.label(actUpgradeAll))
			m.statusErr = false
			m.statusWarn = true
		}
		return m, nil

	case infoMsg:
		if msg.pkg != m.infoPkg || m.running {
			return m, nil // stale response (spinner keeps going), or the run owns Details
		}
		m.loadInfo = false
		text := msg.text
		if msg.err != nil {
			m.status = "info " + msg.pkg + ": " + msg.err.Error()
			m.statusErr = true
			m.statusWarn = false
			text = msg.err.Error() + "\n\n" + text
		}
		m.setDetails("Details: "+msg.pkg, text)
		return m, nil

	case searchMsg:
		if msg.query != m.searchQuery || m.running {
			return m, nil
		}
		m.loadInfo = false
		text := msg.text
		if msg.err != nil {
			m.status = "search " + msg.query + ": " + msg.err.Error()
			m.statusErr = true
			m.statusWarn = false
			text = msg.err.Error() + "\n\n" + text
		} else if strings.TrimSpace(text) == "" {
			text = "no results"
		}
		m.setDetails("Details: search "+msg.query, text)
		return m, nil

	case sudoCachedMsg:
		if !m.checkingSudo {
			return m, nil
		}
		m.checkingSudo = false
		if !msg.ok {
			return m, m.openSudo()
		}
		label, steps := m.pendingLabel, m.pendingSteps
		m.clearPending()
		if len(steps) == 0 {
			return m, nil
		}
		return m, m.beginRun(label, steps)

	case sudoAuthMsg:
		if !m.sudoOpen {
			return m, nil
		}
		m.sudoChecking = false
		if msg.err != nil {
			text, retry := sudoErrText(msg.err)
			if !retry {
				m.closeSudo()
				m.status = "sudo: " + text
				m.statusErr = true
				m.statusWarn = false
				return m, nil
			}
			m.sudoAttempts++
			if m.sudoAttempts >= 3 {
				m.closeSudo()
				m.status = "sudo: 3 failed attempts — " + msg.err.Error()
				m.statusErr = true
				m.statusWarn = false
				return m, nil
			}
			m.sudoErr = text
			m.sudoInput.SetValue("")
			return m, m.sudoInput.Focus()
		}
		label, steps := m.pendingLabel, m.pendingSteps
		m.closeSudo()
		if len(steps) == 0 {
			return m, nil
		}
		return m, m.beginRun(label, steps)

	case runMsg:
		if !m.running {
			return m, nil
		}
		if msg.done {
			return m, m.finishRun(msg.err)
		}
		m.runLines = appendOutput(m.runLines, msg.text, msg.overwrite)
		m.renderRun()
		return m, waitRun(m.runCh)

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	if m.sudoOpen {
		var cmd tea.Cmd
		m.sudoInput, cmd = m.sudoInput.Update(msg)
		return m, cmd
	}
	if m.prompt != promptNone {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

// requestQuit is shared by q and ctrl+c. While a command runs, quitting needs
// confirmation; the confirmed quit waits for the command to finish (killing a
// package manager mid-transaction can corrupt its database), and a further
// press forces the exit.
func (m model) requestQuit() (tea.Model, tea.Cmd) {
	switch {
	case !m.running:
		return m, tea.Quit
	case !m.quitWarned:
		m.quitWarned = true
		m.status = fmt.Sprintf("a command is running — press %s again to quit after it finishes", m.keys.label(actQuit))
	case !m.quitWaiting:
		m.quitWaiting = true
		m.status = fmt.Sprintf("waiting for %s to finish… (%s again forces quit; may corrupt the package database)", m.runLabel, m.keys.label(actQuit))
	default:
		return m, tea.Quit
	}
	m.statusErr = false
	m.statusWarn = true
	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" {
		return m.requestQuit()
	}
	if m.sudoOpen {
		return m.handleSudoKey(msg)
	}
	if m.showHelp {
		switch m.keys.action(key) {
		case actQuit:
			return m.requestQuit()
		case actHelp:
			m.showHelp = false
		case actDown:
			_, off := m.helpWindow()
			m.helpScroll = off + 1
		case actUp:
			_, off := m.helpWindow()
			m.helpScroll = maxInt(0, off-1)
		default:
			if key == "esc" {
				m.showHelp = false
			}
		}
		return m, nil
	}
	m.status = ""
	m.statusErr = false
	m.statusWarn = false

	if m.confirm != confirmNone {
		return m.handleConfirm(key)
	}
	if m.prompt != promptNone {
		switch key {
		case "esc":
			var cmd tea.Cmd
			if m.prompt == promptFilter {
				old, _ := m.currentPackage()
				m.filter = ""
				m.input.SetValue("")
				m.applyFilter()
				cmd = m.infoIfChanged(old.Name)
			}
			m.prompt = promptNone
			m.input.Blur()
			return m, cmd
		case "enter":
			return m.commitPrompt()
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		if m.prompt == promptFilter {
			old, _ := m.currentPackage()
			m.filter = m.input.Value()
			m.applyFilter()
			cmd = tea.Batch(cmd, m.infoIfChanged(old.Name))
		}
		return m, cmd
	}

	switch m.keys.action(key) {
	case actQuit:
		return m.requestQuit()
	case actHelp:
		m.showHelp = true
		m.helpScroll = 0
	case actNextPanel:
		m.focus = (m.focus + 1) % 4
	case actPrevPanel:
		m.focus = (m.focus + 3) % 4
	case actFocusManagers:
		m.focus = focusManagers
	case actFocusPackages:
		m.focus = focusPackages
	case actFocusUpdates:
		m.focus = focusUpdates
	case actFocusDetails:
		m.focus = focusDetails
	case actUp:
		return m.moveBy(-1, 1)
	case actDown:
		return m.moveBy(1, 1)
	case actPageDown:
		return m.moveBy(1, m.pageSize())
	case actPageUp:
		return m.moveBy(-1, m.pageSize())
	case actTop:
		return m.jump(-1)
	case actBottom:
		return m.jump(1)
	case actFilter:
		m.focus = focusPackages
		m.prompt = promptFilter
		m.input.Placeholder = ""
		m.input.SetValue(m.filter)
		m.input.CursorEnd()
		return m, m.input.Focus()
	case actSearch:
		mgr, ok := m.currentManager()
		if !ok {
			return m, nil
		}
		if len(mgr.Search) == 0 {
			m.status = mgr.Name + " does not support search"
			m.statusErr = true
			return m, nil
		}
		m.prompt = promptSearch
		m.input.SetValue("")
		m.input.Placeholder = "package name"
		return m, m.input.Focus()
	case actInstall:
		mgr, ok := m.currentManager()
		if !ok {
			return m, nil
		}
		if len(mgr.Install) == 0 {
			m.status = mgr.Name + " does not support install"
			m.statusErr = true
			return m, nil
		}
		m.prompt = promptInstall
		m.input.SetValue(m.searchQuery)
		m.input.Placeholder = "package name"
		m.input.CursorEnd()
		return m, m.input.Focus()
	case actRemove:
		// Remove the item highlighted in the focused panel, never one from another.
		var name string
		switch m.focus {
		case focusPackages:
			if p, ok := m.currentPackage(); ok {
				name = p.Name
			}
		case focusUpdates:
			if u, ok := m.currentUpdate(); ok {
				name = u.Name
			}
		default:
			m.status = "focus Packages or Updates to remove a package"
			m.statusErr = true
			return m, nil
		}
		if name == "" {
			m.status = "no package selected"
			m.statusErr = true
			return m, nil
		}
		mgr, ok := m.currentManager()
		if !ok || len(mgr.Remove) == 0 {
			m.status = "remove is not supported"
			m.statusErr = true
			return m, nil
		}
		m.confirm = confirmRemove
		m.confirmAt = name
		return m, nil
	case actUpgrade:
		if m.focus == focusUpdates {
			pkg, ok := m.currentUpdate()
			if !ok {
				m.status = "no update selected"
				m.statusErr = true
				return m, nil
			}
			mgr, ok := m.currentManager()
			if !ok || len(mgr.Upgrade) == 0 {
				m.status = "upgrade is not supported"
				m.statusErr = true
				return m, nil
			}
			m.confirm = confirmUpgrade
			m.confirmAt = pkg.Name
			return m, nil
		}
		pkg, ok := m.currentPackage()
		if !ok {
			m.status = "no package selected"
			m.statusErr = true
			return m, nil
		}
		mgr, ok := m.currentManager()
		if !ok || len(mgr.Upgrade) == 0 {
			m.status = "upgrade is not supported"
			m.statusErr = true
			return m, nil
		}
		m.confirm = confirmUpgrade
		m.confirmAt = pkg.Name
		return m, nil
	case actUpgradeAll:
		mgr, ok := m.currentManager()
		if !ok || len(mgr.UpgradeAll) == 0 {
			m.status = "upgrade all is not supported"
			m.statusErr = true
			return m, nil
		}
		m.confirm = confirmUpgradeAll
		return m, nil
	case actUpgradeEverything:
		targets, pending, skipped := upgradeTargets(m.managers, m.updatesCache, m.updatesFailed)
		if len(pending) > 0 {
			m.status = "still checking updates for " + strings.Join(pending, ", ") + "…"
			return m, nil
		}
		if len(skipped) > 0 && !m.running {
			m.setDetails("Details: update check failed", failureReport(skipped, m.updatesFailed))
		}
		if len(targets) == 0 {
			if len(skipped) > 0 {
				m.status = "update check failed for " + strings.Join(skipped, ", ") + " — select it and press " + m.keys.label(actRefresh) + " to retry"
				m.statusErr = true
				return m, nil
			}
			m.status = everythingUpToDate
			return m, nil
		}
		m.confirm = confirmUpgradeEverything
		m.confirmTargets = targets
		m.confirmSkipped = skipped
		return m, nil
	case actRefresh:
		m.status = "refreshing…"
		m.statusErr = false
		m.statusWarn = false
		return m, tea.Batch(m.startPackages(true), m.startUpdates(true))
	case actInfo:
		switch m.focus {
		case focusManagers:
			m.focus = focusPackages
			if len(m.packages) == 0 {
				return m, m.startPackages(false)
			}
			return m, m.packageChanged()
		case focusPackages:
			return m, m.packageChanged()
		case focusUpdates:
			return m, m.updateInfo()
		}
	}
	return m, nil
}

func (m model) handleSudoKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.sudoChecking {
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.closeSudo()
		m.status = "cancelled"
		m.statusErr = false
		m.statusWarn = false
		return m, nil
	case "enter":
		return m.submitSudo()
	}
	var cmd tea.Cmd
	m.sudoInput, cmd = m.sudoInput.Update(msg)
	return m, cmd
}

func (m model) handleConfirm(key string) (tea.Model, tea.Cmd) {
	kind := m.confirm
	if key != "y" && key != "Y" {
		if key == "n" || key == "N" || key == "esc" {
			m.confirm = confirmNone
			m.confirmTargets = nil
			m.status = "cancelled"
		}
		return m, nil
	}
	m.confirm = confirmNone
	targets := m.confirmTargets
	m.confirmTargets = nil
	if kind == confirmUpgradeEverything {
		steps := make([]step, 0, len(targets))
		for _, mgr := range targets {
			steps = append(steps, step{mgr: mgr, cmd: mgr.Cmd(true, mgr.UpgradeAll)})
		}
		return m, m.startMutation("upgrade everything", steps)
	}
	mgr, ok := m.currentManager()
	if !ok {
		return m, nil
	}
	switch kind {
	case confirmRemove:
		label := "remove " + m.confirmAt
		return m, m.startMutation(label, []step{{mgr: mgr, cmd: mgr.Cmd(true, mgr.Remove, m.confirmAt)}})
	case confirmUpgrade:
		label := "upgrade " + m.confirmAt
		return m, m.startMutation(label, []step{{mgr: mgr, cmd: mgr.Cmd(true, mgr.Upgrade, m.confirmAt)}})
	case confirmUpgradeAll:
		return m, m.startMutation("upgrade all", []step{{mgr: mgr, cmd: mgr.Cmd(true, mgr.UpgradeAll)}})
	}
	return m, nil
}

func (m model) commitPrompt() (tea.Model, tea.Cmd) {
	kind := m.prompt
	value := strings.TrimSpace(m.input.Value())
	m.prompt = promptNone
	m.input.Blur()

	switch kind {
	case promptFilter:
		m.filter = m.input.Value()
		m.applyFilter()
		m.status = fmt.Sprintf("%d matching packages", len(m.filtered))
		return m, nil

	case promptSearch:
		mgr, ok := m.currentManager()
		if !ok || value == "" {
			return m, nil
		}
		m.searchQuery = value
		m.infoPkg = ""
		m.loadInfo = true
		m.setDetails("Details: search "+value, "searching…")
		return m, tea.Batch(m.ensureSpin(), searchCmd(mgr, value))

	case promptInstall:
		mgr, ok := m.currentManager()
		if !ok || value == "" {
			m.status = "nothing to install"
			return m, nil
		}
		pkgs := strings.Fields(value)
		label := "install " + strings.Join(pkgs, ", ")
		return m, m.startMutation(label, []step{{mgr: mgr, cmd: mgr.Cmd(true, mgr.Install, pkgs...)}})
	}
	return m, nil
}

func (m model) moveBy(dir, n int) (tea.Model, tea.Cmd) {
	if n < 1 {
		n = 1
	}
	switch m.focus {
	case focusManagers:
		if len(m.managers) == 0 {
			return m, nil
		}
		old := m.mCursor
		m.mCursor = clamp(m.mCursor+dir*n, 0, len(m.managers)-1)
		if m.mCursor != old {
			return m, m.managerChanged()
		}
	case focusPackages:
		if len(m.filtered) == 0 {
			return m, nil
		}
		old := m.pCursor
		m.pCursor = clamp(m.pCursor+dir*n, 0, len(m.filtered)-1)
		if m.pCursor != old {
			return m, m.packageChanged()
		}
	case focusUpdates:
		if len(m.updates) == 0 {
			return m, nil
		}
		m.updCursor = clamp(m.updCursor+dir*n, 0, len(m.updates)-1)
	case focusDetails:
		if dir < 0 {
			m.details.ScrollUp(n)
			if m.running {
				m.runFollow = false
			}
		} else {
			m.details.ScrollDown(n)
		}
	}
	return m, nil
}

func (m model) jump(dir int) (tea.Model, tea.Cmd) {
	switch m.focus {
	case focusManagers:
		if len(m.managers) == 0 {
			return m, nil
		}
		old := m.mCursor
		if dir < 0 {
			m.mCursor = 0
		} else {
			m.mCursor = len(m.managers) - 1
		}
		if m.mCursor != old {
			return m, m.managerChanged()
		}
	case focusPackages:
		if len(m.filtered) == 0 {
			return m, nil
		}
		old := m.pCursor
		if dir < 0 {
			m.pCursor = 0
		} else {
			m.pCursor = len(m.filtered) - 1
		}
		if m.pCursor != old {
			return m, m.packageChanged()
		}
	case focusUpdates:
		if len(m.updates) == 0 {
			return m, nil
		}
		if dir < 0 {
			m.updCursor = 0
		} else {
			m.updCursor = len(m.updates) - 1
		}
	case focusDetails:
		if dir < 0 {
			m.details.GotoTop()
			if m.running {
				m.runFollow = false
			}
		} else {
			m.details.GotoBottom()
			m.runFollow = true
		}
	}
	return m, nil
}

// pageSize is the number of rows a pgup/pgdown moves in the focused panel.
func (m model) pageSize() int {
	l := m.layout()
	switch m.focus {
	case focusManagers:
		return maxInt(1, l.mgrH-3)
	case focusPackages:
		return maxInt(1, l.pkgH-3)
	case focusUpdates:
		return maxInt(1, l.updH-3)
	}
	if m.details.Height > 0 {
		return m.details.Height
	}
	return 1
}
