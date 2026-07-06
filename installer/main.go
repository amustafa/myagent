package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	source := flag.String("source", "", "path to the repo whose .claude/ to install from (default: auto-detect from cwd upward)")
	list := flag.Bool("list", false, "print the discovered components and exit (no TUI)")
	flag.Parse()

	start := *source
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

	comps, err := scanComponents(sourceClaude)
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan failed:", err)
		os.Exit(1)
	}
	if len(comps) == 0 {
		fmt.Fprintln(os.Stderr, "no installable components found under", sourceClaude)
		os.Exit(1)
	}

	if *list {
		fmt.Printf("Components under %s:\n", sourceClaude)
		for _, c := range comps {
			fmt.Printf("  %-10s %s\n", c.Type, c.Name)
		}
		return
	}

	m := newModel(sourceClaude, comps)
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tui error:", err)
		os.Exit(1)
	}
}
