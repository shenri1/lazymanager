package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/shenri1/lazymanager/internal/config"
	"github.com/shenri1/lazymanager/internal/manager"
	"github.com/shenri1/lazymanager/internal/theme"
	"github.com/shenri1/lazymanager/internal/ui"
)

// version is set with -ldflags "-X main.version=..."; otherwise it comes from
// the module version embedded by `go install` (see buildVersion).
var version = ""

func buildVersion() string {
	if version != "" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
		return strings.TrimPrefix(bi.Main.Version, "v") // match the GoReleaser build, which has no "v"
	}
	return "dev"
}

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	dir := flag.String("C", "", "directory to inspect (default: current directory)")
	noProject := flag.Bool("no-project", false, "only system managers; skip project managers")
	themeName := flag.String("theme", "", "theme name (overrides config.toml)")
	listThemes := flag.Bool("list-themes", false, "list available themes and exit")
	printConfig := flag.Bool("print-config", false, "print the default configuration and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("lazymanager " + buildVersion())
		return
	}
	if *printConfig {
		fmt.Print(config.DefaultTOML)
		return
	}
	if *listThemes {
		printThemes()
		return
	}

	cfg, warnings := config.Load()

	name := strings.TrimSpace(cfg.Theme.Name)
	if *themeName != "" {
		name = strings.TrimSpace(*themeName)
	}
	active, terr := theme.Resolve(name, config.ThemesDir())
	if terr != nil {
		warnings = append(warnings, fmt.Sprintf("theme: %v; using %s", terr, config.DefaultTheme))
		active = theme.Default()
	}
	if len(cfg.Theme.Colors) > 0 {
		var errs []error
		active, errs = active.WithOverrides(cfg.Theme.Colors)
		for _, e := range errs {
			warnings = append(warnings, "theme: "+e.Error())
		}
	}
	noColor := os.Getenv("NO_COLOR") != ""

	cwd := *dir
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			fail(err)
		}
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		fail(err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		fail(err)
	}
	if !info.IsDir() {
		fail(fmt.Errorf("%s is not a directory", abs))
	}
	// Project managers run inside the inspected repository.
	if err := os.Chdir(abs); err != nil {
		fail(err)
	}

	managers := manager.Detect(abs)
	if len(cfg.Managers.Disabled) > 0 {
		managers = slices.DeleteFunc(managers, func(m manager.Manager) bool {
			return cfg.IsManagerDisabled(m.Name)
		})
	}
	// Project managers can execute repo-controlled code (Gemfile, .yarnrc,
	// .cargo/config.toml, composer plugins), so gate them behind trust.
	if manager.HasProject(managers) {
		switch {
		case *noProject:
			managers = manager.SystemOnly(managers)
		case !manager.IsTrusted(abs):
			fmt.Printf("%s contains project files. Listing dependencies may run code from this repo.\nTrust this directory? [y/N] ", abs)
			var ans string
			_, _ = fmt.Scanln(&ans)
			if ans == "y" || ans == "Y" {
				if err := manager.Trust(abs); err != nil {
					fmt.Fprintln(os.Stderr, "lazymanager: could not save trust:", err)
				}
			} else {
				managers = manager.SystemOnly(managers)
			}
		}
	}

	program := tea.NewProgram(
		ui.New(abs, managers, ui.Options{Theme: active, Config: cfg, Warnings: warnings, NoColor: noColor}),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := program.Run(); err != nil {
		fail(err)
	}
}

func printThemes() {
	seen := make(map[string]bool)
	for _, name := range theme.Builtins() {
		fmt.Println(name)
		seen[name] = true
	}
	for _, name := range theme.ListUser(config.ThemesDir()) {
		if !seen[name] {
			fmt.Println(name)
			seen[name] = true
		}
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "lazymanager:", err)
	os.Exit(1)
}
