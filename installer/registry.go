package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

// The flavor registry is global (a recipe is environment-independent):
//
//   ${MYAGENTCFG_DIR}/flavors/<instance>/
//     input.json     saved option values — the re-render source of truth
//     meta.json      { skill, source, commit, created_at, updated_at }
//     rendered/      output of install.py (the resolved skill)
//
// Environments symlink rendered/ into <target>/.claude/skills/<instance>/, so
// regenerating rendered/ in place updates every environment that links it.

// mcpRenderedFile is the file an MCP flavor template's install.py must write
// into --dest: the server object as it appears inside "mcpServers".
const mcpRenderedFile = "server.json"

// FlavorMeta is the provenance recorded for a generated flavor.
type FlavorMeta struct {
	Skill     string `json:"skill"`            // source template name, e.g. "orchestrate"
	Target    string `json:"target,omitempty"` // "skill" (default) or "mcp"
	Source    string `json:"source"`           // absolute source template dir at create time
	Commit    string `json:"commit"`           // source repo commit the render was frozen at
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// FlavorInstance is one generated flavor on disk.
type FlavorInstance struct {
	Name  string         // instance name (registry dir)
	Meta  FlavorMeta     //
	Input map[string]any // saved option values
	Dir   string         // flavors/<name>
}

// Rendered is the directory holding the resolved skill for this instance.
func (f FlavorInstance) Rendered() string { return filepath.Join(f.Dir, "rendered") }

// asComponent presents a flavor instance as an installable Component. A skill
// flavor symlinks its rendered dir into skills/<name>; an MCP flavor points at
// the single rendered server.json and installs through the MCP merge path
// (routed by Type == "mcp" in classifyComponent/applyComponent).
func (f FlavorInstance) asComponent() Component {
	inst := f
	if f.Meta.Target == "mcp" {
		return Component{
			Type:    "mcp",
			Label:   "MCP Servers",
			Name:    f.Name + ".json",
			Source:  filepath.Join(f.Rendered(), mcpRenderedFile),
			RelPath: filepath.Join("mcp", f.Name+".json"),
			IsDir:   false,
			Flavor:  &inst,
		}
	}
	return Component{
		Type:    "flavors",
		Label:   "Flavors",
		Name:    f.Name,
		Source:  f.Rendered(),
		RelPath: filepath.Join("skills", f.Name),
		IsDir:   true,
		Flavor:  &inst,
	}
}

// updateAvailable reports whether the instance was rendered at a different
// source commit than currentCommit (empty currentCommit ⇒ can't tell ⇒ false).
func (f FlavorInstance) updateAvailable(currentCommit string) bool {
	return currentCommit != "" && f.Meta.Commit != "" && f.Meta.Commit != currentCommit
}

func flavorsDir() string { return filepath.Join(configBaseDir(), "flavors") }

// listFlavors loads every generated flavor from the registry, sorted by name.
func listFlavors() []FlavorInstance {
	root := flavorsDir()
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []FlavorInstance
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if inst, ok := loadFlavor(filepath.Join(root, e.Name())); ok {
			out = append(out, inst)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func loadFlavor(dir string) (FlavorInstance, bool) {
	inst := FlavorInstance{Name: filepath.Base(dir), Dir: dir}
	mb, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		return inst, false
	}
	if json.Unmarshal(mb, &inst.Meta) != nil {
		return inst, false
	}
	if ib, err := os.ReadFile(filepath.Join(dir, "input.json")); err == nil {
		_ = json.Unmarshal(ib, &inst.Input)
	}
	return inst, true
}

func flavorExists(name string) bool {
	_, err := os.Stat(filepath.Join(flavorsDir(), name))
	return err == nil
}

// environmentsUsingFlavor returns the target paths of every environment whose
// manifest still records an install of the named flavor. Deleting a flavor from
// the global registry would leave those environments with dangling symlinks, so
// the TUI warns about them first.
func environmentsUsingFlavor(name string) []string {
	envRoot := filepath.Join(configBaseDir(), "environments")
	entries, err := os.ReadDir(envRoot)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(envRoot, e.Name(), "installed.json"))
		if err != nil {
			continue
		}
		var man Manifest
		if json.Unmarshal(data, &man) != nil {
			continue
		}
		for _, rec := range man.Instances {
			if rec.Kind == "flavored" && rec.Options["flavor"] == name {
				out = append(out, man.Target)
				break
			}
		}
	}
	return out
}

// createFlavor generates a new flavor: renders via the template's install.py and
// writes input.json + meta.json. It does not install into any environment.
func createFlavor(tpl Template, name string, input map[string]any, commit string) (FlavorInstance, error) {
	if name == "" {
		return FlavorInstance{}, fmt.Errorf("flavor name is required")
	}
	if flavorExists(name) {
		return FlavorInstance{}, fmt.Errorf("a flavor named %q already exists", name)
	}
	dir := filepath.Join(flavorsDir(), name)
	inst := FlavorInstance{
		Name: name, Dir: dir, Input: input,
		Meta: FlavorMeta{
			Skill: tpl.Name, Target: tpl.Target, Source: tpl.Dir,
			Commit: commit, CreatedAt: nowStamp(),
		},
	}
	if err := renderFlavor(tpl.Dir, inst); err != nil {
		os.RemoveAll(dir) // don't leave a half-written instance
		return FlavorInstance{}, err
	}
	if err := writeFlavorFiles(inst); err != nil {
		os.RemoveAll(dir)
		return FlavorInstance{}, err
	}
	return inst, nil
}

// updateFlavor re-renders an existing flavor from its saved input against the
// current source, stamping the new commit. Environments symlinking rendered/
// pick up the change automatically.
func updateFlavor(inst FlavorInstance, commit string) (FlavorInstance, error) {
	if err := renderFlavor(inst.Meta.Source, inst); err != nil {
		return inst, err
	}
	inst.Meta.Commit = commit
	inst.Meta.UpdatedAt = nowStamp()
	if err := writeFlavorFiles(inst); err != nil {
		return inst, err
	}
	return inst, nil
}

// editFlavor re-renders an existing flavor with *new* option values (vs
// updateFlavor, which re-renders the same saved input against new skill code).
// It stamps the current commit since the render reflects the current source.
func editFlavor(inst FlavorInstance, skillDir string, input map[string]any, commit string) (FlavorInstance, error) {
	inst.Input = input
	if err := renderFlavor(skillDir, inst); err != nil {
		return inst, err
	}
	inst.Meta.Commit = commit
	inst.Meta.UpdatedAt = nowStamp()
	if err := writeFlavorFiles(inst); err != nil {
		return inst, err
	}
	return inst, nil
}

// deleteFlavor removes the whole registry entry. Uninstalling symlinks from
// environments is the caller's responsibility.
func deleteFlavor(name string) error {
	return os.RemoveAll(filepath.Join(flavorsDir(), name))
}

// renderFlavor runs the skill's install.py to (re)generate rendered/. The chosen
// option values are handed to the script as JSON on stdin; the script writes the
// resolved skill into --dest. Rendering is atomic-ish: a fresh temp dir is
// populated, then swapped in for the previous rendered/.
func renderFlavor(skillDir string, inst FlavorInstance) error {
	if err := os.MkdirAll(inst.Dir, 0o755); err != nil {
		return err
	}
	tmp := inst.Rendered() + ".new"
	os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	payload, err := json.Marshal(inst.Input)
	if err != nil {
		return err
	}
	cmd := exec.Command("python3", filepath.Join(skillDir, flavorRenderFile), "--dest", tmp)
	cmd.Stdin = bytes.NewReader(payload)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmp)
		msg := stderr.String()
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("install.py failed: %s", msg)
	}
	final := inst.Rendered()
	os.RemoveAll(final)
	return os.Rename(tmp, final)
}

func writeFlavorFiles(inst FlavorInstance) error {
	if err := writeJSON(filepath.Join(inst.Dir, "meta.json"), inst.Meta); err != nil {
		return err
	}
	return writeJSON(filepath.Join(inst.Dir, "input.json"), inst.Input)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
