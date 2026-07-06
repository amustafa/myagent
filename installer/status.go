package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// The -status report is a headless, read-only view of install state: every
// environment's reconciled manifest plus the global flavor registry. It mirrors
// -list in spirit (plain text, no TUI) but reads the store rather than the repo.

// kindRank orders the instance kinds within an environment's listing so the
// output is stable regardless of map iteration order.
var kindRank = map[string]int{"symlink": 0, "flavored": 1, "mcp": 2}

// loadManifestFile reads a specific installed.json, then reconciles it against
// the filesystem so the report reflects reality (dangling entries are pruned).
// Unlike loadManifest it takes the file path directly, letting the report walk
// every environment dir without re-deriving each target's slug.
func loadManifestFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m := &Manifest{Instances: map[string]InstanceRecord{}, path: path}
	if err := json.Unmarshal(data, m); err != nil {
		return nil, err
	}
	if m.Instances == nil {
		m.Instances = map[string]InstanceRecord{}
	}
	m.reconcile()
	return m, nil
}

// loadAllManifests reads every environment manifest under
// ${MYAGENTCFG_DIR}/environments/*/installed.json, reconciled, sorted by target.
func loadAllManifests() []*Manifest {
	envRoot := filepath.Join(configBaseDir(), "environments")
	entries, err := os.ReadDir(envRoot)
	if err != nil {
		return nil // no environments dir — nothing installed yet
	}
	var out []*Manifest
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(envRoot, e.Name(), "installed.json")
		m, err := loadManifestFile(p)
		if err != nil {
			continue // absent or corrupt manifest — skip this env
		}
		out = append(out, m)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Target < out[j].Target })
	return out
}

// envSlugOf recovers the environment slug (the directory name) from a loaded
// manifest's on-disk path.
func envSlugOf(m *Manifest) string {
	return filepath.Base(filepath.Dir(m.path))
}

// writeStatus renders the full status report to w. sourceClaude locates the
// source repo so flavor drift can be judged against its current commit.
func writeStatus(w io.Writer, sourceClaude string) {
	manifests := loadAllManifests()
	flavors := listFlavors()
	currentCommit := sourceCommit(filepath.Dir(sourceClaude))

	total := 0
	for _, m := range manifests {
		total += len(m.Instances)
	}
	if total == 0 && len(flavors) == 0 {
		fmt.Fprintln(w, "Nothing installed yet. Run the installer to add components.")
		return
	}

	// Align the rel-path column across every instance in every environment.
	relWidth := 0
	for _, m := range manifests {
		for rel := range m.Instances {
			if len(rel) > relWidth {
				relWidth = len(rel)
			}
		}
	}

	fmt.Fprintln(w, "Environments:")
	if len(manifests) == 0 {
		fmt.Fprintln(w, "  (none)")
	}
	for _, m := range manifests {
		fmt.Fprintf(w, "\n  %s → %s\n", envSlugOf(m), m.Target)
		if len(m.Instances) == 0 {
			fmt.Fprintln(w, "    (nothing installed)")
			continue
		}
		// Sort instances by kind, then rel path, for a stable listing.
		rels := make([]string, 0, len(m.Instances))
		for rel := range m.Instances {
			rels = append(rels, rel)
		}
		sort.SliceStable(rels, func(i, j int) bool {
			ri, rj := m.Instances[rels[i]], m.Instances[rels[j]]
			if kindRank[ri.Kind] != kindRank[rj.Kind] {
				return kindRank[ri.Kind] < kindRank[rj.Kind]
			}
			return rels[i] < rels[j]
		})
		for _, rel := range rels {
			rec := m.Instances[rel]
			fmt.Fprintf(w, "    %-9s %-*s  %s\n", rec.Kind, relWidth, rel, rec.InstalledAt)
		}
	}

	fmt.Fprintln(w, "\nFlavors:")
	if len(flavors) == 0 {
		fmt.Fprintln(w, "  (none)")
		return
	}
	for _, f := range flavors {
		commit := f.Meta.Commit
		if commit == "" {
			commit = "?"
		}
		fmt.Fprintf(w, "  %-20s skill=%-14s commit=%-10s %s\n",
			f.Name, f.Meta.Skill, commit, flavorDrift(f, currentCommit))
	}
}

// flavorDrift describes a flavor's freshness relative to the source's current
// commit: whether it's current, stale, or unknowable (no source commit).
func flavorDrift(f FlavorInstance, currentCommit string) string {
	if currentCommit == "" {
		return "(source commit unknown)"
	}
	if f.updateAvailable(currentCommit) {
		return "update available"
	}
	return "up to date"
}
