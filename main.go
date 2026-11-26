package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/shenri1/lazymanager/internal/manager"
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
		return bi.Main.Version
	}
	return "dev"
}

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	dir := flag.String("C", "", "directory to inspect (default: current directory)")
	noProject := flag.Bool("no-project", false, "only system managers; skip project managers")
	flag.Parse()

	if *showVersion {
		fmt.Println("lazymanager " + buildVersion())
		return
	}

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

	program := tea.NewProgram(ui.New(abs, managers), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := program.Run(); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "lazymanager:", err)
	os.Exit(1)
}
