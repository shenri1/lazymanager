package ui

import tea "github.com/charmbracelet/bubbletea"

// panelAt maps a screen coordinate to a panel and to the list row under the
// cursor. Row -1 means the click landed on the panel title or on empty space;
// ok is false outside every panel (including borders and the status bar).
func (m model) panelAt(x, y int) (panel int, row int, ok bool) {
	if m.width <= 0 || m.height <= 0 {
		return 0, 0, false
	}
	if x < 0 || y < 0 || x >= m.width || y >= m.height-1 {
		return 0, 0, false
	}
	l := m.layout()
	if x >= l.leftW {
		relX := x - l.leftW
		if relX <= 0 || relX >= l.rightW-1 || y == 0 || y >= l.bodyH-1 {
			return focusDetails, 0, false
		}
		if y == 1 {
			return focusDetails, -1, true
		}
		return focusDetails, y - 2, true
	}
	switch {
	case y < l.mgrH:
		return m.listHit(focusManagers, x, y, l.leftW, l.mgrH, m.mCursor, len(m.managers))
	case y < l.mgrH+l.pkgH:
		return m.listHit(focusPackages, x, y-l.mgrH, l.leftW, l.pkgH, m.pCursor, len(m.filtered))
	default:
		return m.listHit(focusUpdates, x, y-l.mgrH-l.pkgH, l.leftW, l.updH, m.updCursor, len(m.updates))
	}
}

func (m model) listHit(panel, x, y, w, h, cursor, total int) (int, int, bool) {
	if x <= 0 || x >= w-1 || y == 0 || y >= h-1 {
		return panel, 0, false
	}
	if y == 1 {
		return panel, -1, true
	}
	visible := h - 3
	if visible < 0 {
		visible = 0
	}
	content := y - 2
	if content < 0 || content >= visible {
		return panel, -1, true
	}
	row := scrollStart(cursor, total, visible) + content
	if row >= total {
		return panel, -1, true
	}
	return panel, row, true
}

func (m model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Modal dialogs, prompts, confirmations and help own the screen; ignore
	// all mouse input.
	if m.sudoOpen || m.showHelp || m.prompt != promptNone || m.confirm != confirmNone {
		return m, nil
	}

	if msg.Action != tea.MouseActionPress {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		panel, _, ok := m.panelAt(msg.X, msg.Y)
		if !ok {
			return m, nil
		}
		return m.wheel(panel, -1)
	case tea.MouseButtonWheelDown:
		panel, _, ok := m.panelAt(msg.X, msg.Y)
		if !ok {
			return m, nil
		}
		return m.wheel(panel, 1)
	}

	if msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	panel, row, ok := m.panelAt(msg.X, msg.Y)
	if !ok {
		return m, nil
	}
	m.focus = panel
	if row < 0 {
		return m, nil
	}

	switch panel {
	case focusManagers:
		if row >= 0 && row < len(m.managers) && row != m.mCursor {
			m.mCursor = row
			return m, m.managerChanged()
		}
	case focusPackages:
		if row >= 0 && row < len(m.filtered) && row != m.pCursor {
			m.pCursor = row
			return m, m.packageChanged()
		}
	case focusUpdates:
		if row >= 0 && row < len(m.updates) {
			m.updCursor = row
		}
	}
	return m, nil
}

// wheel moves a panel's selection by one row, or scrolls Details by three
// lines, without changing focus.
func (m model) wheel(panel, dir int) (tea.Model, tea.Cmd) {
	switch panel {
	case focusManagers:
		if len(m.managers) == 0 {
			return m, nil
		}
		old := m.mCursor
		m.mCursor = clamp(m.mCursor+dir, 0, len(m.managers)-1)
		if m.mCursor != old {
			return m, m.managerChanged()
		}
	case focusPackages:
		if len(m.filtered) == 0 {
			return m, nil
		}
		old := m.pCursor
		m.pCursor = clamp(m.pCursor+dir, 0, len(m.filtered)-1)
		if m.pCursor != old {
			return m, m.packageChanged()
		}
	case focusUpdates:
		if len(m.updates) == 0 {
			return m, nil
		}
		m.updCursor = clamp(m.updCursor+dir, 0, len(m.updates)-1)
	case focusDetails:
		if dir < 0 {
			m.details.ScrollUp(3)
			if m.running {
				m.runFollow = false
			}
		} else {
			m.details.ScrollDown(3)
		}
	}
	return m, nil
}
