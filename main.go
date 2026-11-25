package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"lazymanager/internal/manager"
	"lazymanager/internal/ui"
)

const version = "0.1.0"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	dir := flag.String("C", "", "directory to inspect (default: current directory)")
	flag.Parse()

	if *showVersion {
		fmt.Println("lazymanager " + version)
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

	program := tea.NewProgram(ui.New(abs, manager.Detect(abs)), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := program.Run(); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "lazymanager:", err)
	os.Exit(1)
}
