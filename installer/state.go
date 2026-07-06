package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// nonSlugChars matches any run of characters that aren't safe in a directory name.
var nonSlugChars = regexp.MustCompile(`[^A-Za-z0-9]+`)

// nowStamp is the install timestamp format used in manifest records.
func nowStamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// The install-state store lives under ${MYAGENTCFG_DIR:-~/.config/myagentcfg}
// and is partitioned per environment, where an environment is one install
// target (global ~/.claude, or a project's .claude). Each environment has an
// installed.json manifest recording what this tool put there.

// InstanceRecord is one installed component in a manifest.
type InstanceRecord struct {
	Kind        string         `json:"kind"`              // "symlink" (stage 1) | "flavored" (stage 2)
	Source      string         `json:"source"`            // absolute source path
	InstalledAt string         `json:"installed_at"`      // RFC3339-ish stamp
	Commit      string         `json:"commit,omitempty"`  // source repo commit at install time
	Options     map[string]any `json:"options,omitempty"` // flavor choices (stage 2)
}

// Manifest is the per-environment record of installs. It is a *record*, not a
// source of truth: reconcile() prunes entries whose symlink no longer exists so
// the file can't drift into lying about reality.
type Manifest struct {
	Target    string                    `json:"target"` // the .claude root this env installs into
	Instances map[string]InstanceRecord `json:"instances"`

	path string // on-disk location; not serialized
}

// configBaseDir resolves the store root: $MYAGENTCFG_DIR, else ~/.config/myagentcfg.
func configBaseDir() string {
	if d := strings.TrimSpace(os.Getenv("MYAGENTCFG_DIR")); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".myagentcfg" // last-resort relative fallback
	}
	return filepath.Join(home, ".config", "myagentcfg")
}

// envSlug names the environment for a target .claude root. The global namespace
// is "global"; a project is a filesystem-safe slug of its root path.
func envSlug(targetClaude string) string {
	home, _ := os.UserHomeDir()
	if home != "" && targetClaude == filepath.Join(home, ".claude") {
		return "global"
	}
	root := filepath.Dir(targetClaude) // the project root holding .claude
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	slug := nonSlugChars.ReplaceAllString(abs, "_")
	slug = strings.Trim(slug, "_")
	if slug == "" {
		slug = "root"
	}
	return slug
}

// sourceCommit returns the short commit of the source repo, or "" if unavailable
// (not a git repo, git missing). A dirty tree gets a "-dirty" suffix so an
// install from uncommitted work is distinguishable on later drift checks.
func sourceCommit(sourceRepoRoot string) string {
	out, err := exec.Command("git", "-C", sourceRepoRoot, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	commit := strings.TrimSpace(string(out))
	if commit == "" {
		return ""
	}
	if st, err := exec.Command("git", "-C", sourceRepoRoot, "status", "--porcelain").Output(); err == nil {
		if strings.TrimSpace(string(st)) != "" {
			commit += "-dirty"
		}
	}
	return commit
}

// loadManifest reads (or initializes) the manifest for a target, then reconciles
// it against the filesystem.
func loadManifest(targetClaude string) *Manifest {
	base := configBaseDir()
	p := filepath.Join(base, "environments", envSlug(targetClaude), "installed.json")
	m := &Manifest{Target: targetClaude, Instances: map[string]InstanceRecord{}, path: p}
	if data, err := os.ReadFile(p); err == nil {
		_ = json.Unmarshal(data, m) // a corrupt file just yields an empty manifest
		if m.Instances == nil {
			m.Instances = map[string]InstanceRecord{}
		}
		m.Target = targetClaude
		m.path = p
	}
	m.reconcile()
	return m
}

// reconcile drops any recorded instance whose destination is no longer a symlink
// (hand-deleted, replaced by a real file, etc.), keeping the record honest.
func (m *Manifest) reconcile() {
	for rel, rec := range m.Instances {
		if rec.Kind == "mcp" {
			// MCP installs aren't symlinks — verify the server is still in the
			// target's MCP config instead.
			if !mcpServerPresent(m.Target, rel) {
				delete(m.Instances, rel)
			}
			continue
		}
		dest := filepath.Join(m.Target, rel)
		info, err := os.Lstat(dest)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			delete(m.Instances, rel)
		}
	}
}

// record adds/updates an instance entry.
func (m *Manifest) record(rel string, rec InstanceRecord) {
	m.Instances[rel] = rec
}

// forget removes an instance entry (uninstall).
func (m *Manifest) forget(rel string) {
	delete(m.Instances, rel)
}

// save writes the manifest atomically, creating parent dirs as needed.
func (m *Manifest) save() error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, m.path)
}
