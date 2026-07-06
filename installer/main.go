package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// defaultSource is baked in at build time via
// -ldflags "-X main.defaultSource=<repo>" (see `make install`). It lets an
// installed `myagent` binary install *from* the repo it was built from,
// regardless of the working directory. `go run` leaves it empty and falls back
// to walking up from the cwd.
var defaultSource string

func main() {
	source := flag.String("source", "", "path to the repo whose .claude/ to install from (default: the repo this binary was built from, else auto-detect from cwd)")
	list := flag.Bool("list", false, "print the discovered components and exit (no TUI)")
	status := flag.Bool("status", false, "print a read-only report of install state and exit (no TUI)")
	flag.Parse()

	start := *source
	if start == "" {
		start = defaultSource // baked at install time; empty under `go run`
	}
	if start == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, "cannot determine working directory:", err)
			os.Exit(1)
		}
		start = cwd
	}

	sourceClaude, err := findSourceClaude(start)
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not find a .claude/ directory at or above", start)
		os.Exit(1)
	}

	// -status reports install state from the store, independent of what the
	// source repo currently offers, so it short-circuits before the scan.
	if *status {
		writeStatus(os.Stdout, sourceClaude)
		return
	}

	comps, err := scanComponents(sourceClaude)
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan failed:", err)
		os.Exit(1)
	}
	templates, _ := scanTemplates(sourceClaude)
	if len(comps) == 0 && len(templates) == 0 {
		fmt.Fprintln(os.Stderr, "no installable components found under", sourceClaude)
		os.Exit(1)
	}

	if *list {
		fmt.Printf("Components under %s:\n", sourceClaude)
		for _, c := range comps {
			fmt.Printf("  %-10s %s\n", c.Type, c.Name)
		}
		for _, t := range templates {
			label := "skills"
			if t.Target == "mcp" {
				label = "mcp"
			}
			fmt.Printf("  %-10s %s (flavorable)\n", label, t.Name)
		}
		return
	}

	m := newModel(sourceClaude, comps, templates)
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tui error:", err)
		os.Exit(1)
	}
}
