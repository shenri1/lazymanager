package ui

import (
	"strings"

	"github.com/shenri1/lazymanager/internal/config"
)

type action int

const (
	actNone action = iota
	actQuit
	actHelp
	actRefresh
	actUp
	actDown
	actPageUp
	actPageDown
	actTop
	actBottom
	actNextPanel
	actPrevPanel
	actFocusManagers
	actFocusPackages
	actFocusUpdates
	actFocusDetails
	actFilter
	actSearch
	actInstall
	actRemove
	actUpgrade
	actUpgradeAll
	actUpgradeEverything
	actInfo
)

var actionByName = map[string]action{
	"quit":               actQuit,
	"help":               actHelp,
	"refresh":            actRefresh,
	"up":                 actUp,
	"down":               actDown,
	"page_up":            actPageUp,
	"page_down":          actPageDown,
	"top":                actTop,
	"bottom":             actBottom,
	"next_panel":         actNextPanel,
	"prev_panel":         actPrevPanel,
	"focus_managers":     actFocusManagers,
	"focus_packages":     actFocusPackages,
	"focus_updates":      actFocusUpdates,
	"focus_details":      actFocusDetails,
	"filter":             actFilter,
	"search":             actSearch,
	"install":            actInstall,
	"remove":             actRemove,
	"upgrade":            actUpgrade,
	"upgrade_all":        actUpgradeAll,
	"upgrade_everything": actUpgradeEverything,
	"info":               actInfo,
}

type keymap struct {
	byKey    map[string]action
	byAction map[action][]string
}

func newKeymap(bindings map[string]config.KeyList) keymap {
	km := keymap{
		byKey:    make(map[string]action, len(bindings)),
		byAction: make(map[action][]string, len(bindings)),
	}
	for _, name := range config.Actions() {
		a, ok := actionByName[name]
		if !ok {
			continue
		}
		keys := bindings[name]
		km.byAction[a] = append([]string(nil), keys...)
		for _, key := range keys {
			if _, taken := km.byKey[key]; !taken {
				km.byKey[key] = a
			}
		}
	}
	return km
}

func (k keymap) action(key string) action {
	return k.byKey[key]
}

func (k keymap) label(a action) string {
	keys := k.byAction[a]
	if len(keys) == 0 {
		return ""
	}
	return displayKey(keys[0])
}

func (k keymap) keysLabel(a action) string {
	keys := k.byAction[a]
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, displayKey(key))
	}
	return strings.Join(out, " / ")
}

func (k keymap) hint(actions ...action) string {
	var parts []string
	for _, a := range actions {
		if label := k.label(a); label != "" {
			parts = append(parts, label)
		}
	}
	return strings.Join(parts, "/")
}

func displayKey(key string) string {
	switch key {
	case "right":
		return "→"
	case "left":
		return "←"
	case "up":
		return "↑"
	case "down":
		return "↓"
	}
	return key
}

// helpText is generated from the active keymap so remapped keys are always
// documented correctly.
func helpText(k keymap) string {
	lines := []struct {
		keys string
		desc string
	}{
		{k.keysLabel(actNextPanel), "next panel"},
		{k.keysLabel(actPrevPanel), "previous panel"},
		{k.keysLabel(actFocusManagers), "focus managers"},
		{k.keysLabel(actFocusPackages), "focus packages"},
		{k.keysLabel(actFocusUpdates), "focus updates"},
		{k.keysLabel(actFocusDetails), "focus details"},
		{k.keysLabel(actUp), "move up"},
		{k.keysLabel(actDown), "move down"},
		{k.keysLabel(actPageUp), "move a page up"},
		{k.keysLabel(actPageDown), "move a page down"},
		{k.keysLabel(actTop), "jump to top"},
		{k.keysLabel(actBottom), "jump to bottom"},
		{k.keysLabel(actFilter), "filter packages"},
		{k.keysLabel(actSearch), "search remote"},
		{k.keysLabel(actInstall), "install"},
		{k.keysLabel(actRemove), "remove selected package"},
		{k.keysLabel(actUpgrade), "upgrade selected package or update"},
		{k.keysLabel(actUpgradeAll), "upgrade all"},
		{k.keysLabel(actUpgradeEverything), "upgrade everything"},
		{k.keysLabel(actRefresh), "refresh packages and updates"},
		{k.keysLabel(actInfo), "load info"},
		{k.keysLabel(actHelp), "close help"},
		{k.keysLabel(actQuit), "quit"},
	}

	var b strings.Builder
	b.WriteString("lazymanager — keys\n\n")
	for _, line := range lines {
		if line.keys == "" {
			continue
		}
		b.WriteString("  ")
		b.WriteString(padRight(line.keys, 18))
		b.WriteString(line.desc)
		b.WriteString("\n")
	}
	b.WriteString("\n  mouse              click to focus/select, wheel to move/scroll\n")
	b.WriteString("  esc                cancel prompt / clear filter")
	return b.String()
}
