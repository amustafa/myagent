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
	{dir: "mcp", label: "MCP Servers"},
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

	// Flavor is set when this component is a generated flavor instance rather
	// than a basic repo component; its Source points at the registry render dir.
	Flavor *FlavorInstance
}

// Template is a flavorable source skill: one that ships install.py + flavor.json.
// Templates never install directly — they seed generated flavors via the
// "Add new flavor" flow.
type Template struct {
	Name   string
	Dir    string        // absolute source skill dir
	Schema *FlavorSchema // parsed flavor.json
}

// isFlavorable reports whether a skill directory carries both the render script
// and the schema, marking it a flavor template rather than a basic component.
func isFlavorable(dir string) bool {
	_, e1 := os.Stat(filepath.Join(dir, flavorRenderFile))
	_, e2 := os.Stat(filepath.Join(dir, flavorSchemaFile))
	return e1 == nil && e2 == nil
}

// scanTemplates finds flavorable source skills (those shipping install.py +
// flavor.json) and parses their schemas. A skill whose schema fails to parse is
// skipped rather than aborting the whole scan.
func scanTemplates(sourceClaude string) ([]Template, error) {
	skillsDir := filepath.Join(sourceClaude, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil, nil // no skills dir — no templates
	}
	var out []Template
	for _, e := range entries {
		if !e.IsDir() || e.Name()[0] == '.' {
			continue
		}
		dir := filepath.Join(skillsDir, e.Name())
		if !isFlavorable(dir) {
			continue
		}
		schema, err := parseFlavorSchema(dir)
		if err != nil {
			continue
		}
		out = append(out, Template{Name: e.Name(), Dir: dir, Schema: schema})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
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
			full := filepath.Join(typeDir, name)
			if e.IsDir() && isFlavorable(full) {
				continue // a flavor template, surfaced via scanTemplates instead
			}
			// MCP servers are declared as .json files; skip a README or any
			// other doc that lives alongside them in .claude/mcp/.
			if t.dir == "mcp" && filepath.Ext(name) != ".json" {
				continue
			}
			out = append(out, Component{
				Type:    t.dir,
				Label:   t.label,
				Name:    name,
				Source:  full,
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
