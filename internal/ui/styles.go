package ui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/shenri1/lazymanager/internal/theme"
)

// styles are built once from the active theme. Nothing in the UI should use a
// hard-coded color.
type styles struct {
	base lipgloss.Style

	panelFocused lipgloss.Style
	panelDim     lipgloss.Style

	title     lipgloss.Style
	warnTitle lipgloss.Style
	okTitle   lipgloss.Style
	errTitle  lipgloss.Style

	selectedFocused  lipgloss.Style
	selectedInactive lipgloss.Style

	dim     lipgloss.Style
	err     lipgloss.Style
	status  lipgloss.Style
	warn    lipgloss.Style
	allGood lipgloss.Style

	sudoBox lipgloss.Style

	icons icons
	paint bool
}

type icons struct {
	pointer     string
	managers    string
	packages    string
	updates     string
	details     string
	updateBadge string
	failure     string
	check       string
	warning     string
	error       string
}

func newStyles(th theme.Theme, selectionStyle string, nerdFonts, noColor, paintBackground bool) styles {
	color := func(value string) lipgloss.TerminalColor {
		if noColor || value == "" {
			return lipgloss.NoColor{}
		}
		return lipgloss.Color(value)
	}

	base := lipgloss.NewStyle()
	paint := !noColor && paintBackground && th.Background != ""
	if paint {
		base = base.Background(lipgloss.Color(th.Background))
	}
	if !noColor && th.Foreground != "" {
		base = base.Foreground(lipgloss.Color(th.Foreground))
	}

	focusedBorder, dimBorder := lipgloss.RoundedBorder(), lipgloss.RoundedBorder()
	if noColor {
		// Without color, a different border shape is the only cue for the
		// focused panel.
		focusedBorder = lipgloss.DoubleBorder()
	}

	s := styles{
		base:         base,
		panelFocused: base.Border(focusedBorder).BorderForeground(color(th.BorderActive)),
		panelDim:     base.Border(dimBorder).BorderForeground(color(th.BorderInactive)),
		title:        base.Bold(true).Foreground(color(th.Title)),
		warnTitle:    base.Bold(true).Foreground(color(th.Warning)),
		okTitle:      base.Bold(true).Foreground(color(th.Success)),
		errTitle:     base.Bold(true).Foreground(color(th.Error)),
		dim:          base.Foreground(color(th.Dim)),
		err:          base.Foreground(color(th.Error)),
		status:       base.Foreground(color(th.Dim)),
		warn:         base.Foreground(color(th.Warning)),
		allGood:      base.Foreground(color(th.Success)),
		sudoBox:      base.Border(lipgloss.RoundedBorder()).BorderForeground(color(th.Warning)).Padding(1, 2).Width(46),
		paint:        paint,
		icons:        iconSet(nerdFonts),
	}

	// With NO_COLOR there is no SelectedBg to use; reverse video marks the
	// focused selection and the pointer alone marks the inactive one, so focus
	// stays visible without relying on color.
	if noColor {
		s.selectedFocused = base.Reverse(true)
		s.selectedInactive = base
	} else if selectionStyle == "reverse" || th.SelectedBg == "" {
		s.selectedFocused = base.Reverse(true)
		s.selectedInactive = base.Reverse(true).Foreground(color(th.BorderInactive))
	} else {
		s.selectedFocused = base.Foreground(color(th.SelectedFg)).Background(color(th.SelectedBg))
		s.selectedInactive = base.Foreground(color(th.BorderInactive)).Background(color(th.SelectedBg))
	}
	return s
}

func (s styles) selection(focused bool) lipgloss.Style {
	if focused {
		return s.selectedFocused
	}
	return s.selectedInactive
}

func (s styles) panel(focused bool) lipgloss.Style {
	if focused {
		return s.panelFocused
	}
	return s.panelDim
}

// paintScreen applies the theme background/foreground to the whole frame when
// paint_background is enabled.
func (s styles) paintScreen(view string) string {
	if !s.paint {
		return view
	}
	return s.base.Render(view)
}

func (s styles) titleWith(icon, text string) string {
	if icon == "" {
		return text
	}
	return icon + " " + text
}

func iconSet(nerd bool) icons {
	if !nerd {
		return icons{
			pointer:     "▸",
			updateBadge: "↑",
			failure:     "!",
			check:       "✓",
			warning:     "⚠",
			error:       "✗",
		}
	}
	return icons{
		pointer:     "\uf054", // chevron-right
		managers:    "\uf085", // cogs
		packages:    "\uf1b2", // cube
		updates:     "\uf021", // refresh
		details:     "\uf05a", // info-circle
		updateBadge: "\uf062", // arrow-up
		failure:     "\uf00d", // times
		check:       "\uf00c", // check
		warning:     "\uf071", // exclamation-triangle
		error:       "\uf00d", // times
	}
}
