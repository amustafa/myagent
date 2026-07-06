package main

import (
	"os"
	"path/filepath"
	"sort"
)

// componentType describes a category of installable artifact that lives under
// a repo's .claude/ directory. Each type maps to a subdirectory whose direct
// children are individual components.
type componentType struct {
	dir   string // subdirectory under .claude/ (e.g. "skills")
	label string // human label shown as a group header in the TUI
}

// knownTypes is the set of component categories the installer understands.
// The scanner silently ignores any that don't exist in a given repo, so
// adding a new category here is safe even if most repos lack it.
var knownTypes = []componentType{
	{dir: "skills", label: "Skills"},
	{dir: "commands", label: "Commands"},
	{dir: "agents", label: "Agents"},
	{dir: "hooks", label: "Hooks"},
	{dir: "prompts", label: "Prompts"},
}

// Component is a single installable unit: one skill directory, one command
// file, etc. Source is its absolute path in the repo; RelPath is its path
// relative to .claude/ (e.g. "skills/grill-me"), which is reused verbatim to
// build the destination under any target .claude/ root.
type Component struct {
	Type    string // the type dir, e.g. "skills"
	Label   string // group label, e.g. "Skills"
	Name    string // leaf name, e.g. "grill-me"
	Source  string // absolute source path
	RelPath string // path relative to .claude/, e.g. "skills/grill-me"
	IsDir   bool
}

// findSourceClaude walks up from start looking for a directory that contains a
// ".claude" subdirectory, returning the path to that .claude dir. This lets
// the installer be launched from anywhere inside the myagent repo.
func findSourceClaude(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, ".claude")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

// scanComponents discovers every installable component under sourceClaude.
// Hidden entries (dotfiles) and the SKILL/README metadata that lives inside a
// component are not treated as separate components — only the direct children
// of each type directory are.
func scanComponents(sourceClaude string) ([]Component, error) {
	var out []Component
	for _, t := range knownTypes {
		typeDir := filepath.Join(sourceClaude, t.dir)
		entries, err := os.ReadDir(typeDir)
		if err != nil {
			continue // type dir absent in this repo — fine
		}
		for _, e := range entries {
			name := e.Name()
			if name == "" || name[0] == '.' {
				continue
			}
			out = append(out, Component{
				Type:    t.dir,
				Label:   t.label,
				Name:    name,
				Source:  filepath.Join(typeDir, name),
				RelPath: filepath.Join(t.dir, name),
				IsDir:   e.IsDir(),
			})
		}
	}
	// Stable order: group by type (in knownTypes order), then name.
	typeRank := map[string]int{}
	for i, t := range knownTypes {
		typeRank[t.dir] = i
	}
	sort.SliceStable(out, func(i, j int) bool {
		if typeRank[out[i].Type] != typeRank[out[j].Type] {
			return typeRank[out[i].Type] < typeRank[out[j].Type]
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}
