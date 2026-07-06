package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// seedFlavor writes a minimal flavor registry entry (just meta.json, which is
// all listFlavors needs) so the status report has something to list.
func seedFlavor(t *testing.T, name, skill, commit string) {
	t.Helper()
	dir := filepath.Join(configBaseDir(), "flavors", name)
	mustMkdir(t, dir)
	mustWrite(t, filepath.Join(dir, "meta.json"),
		fmt.Sprintf(`{"skill":%q,"source":"/src","commit":%q,"created_at":"2026-01-01T00:00:00Z"}`,
			skill, commit))
}

// gitCommit turns dir into a git repo with one commit and returns the short hash
// sourceCommit would report, so drift against a known commit can be tested.
func gitCommit(t *testing.T, dir string) string {
	t.Helper()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
		{"add", "-A"},
		{"commit", "-m", "seed"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return sourceCommit(dir)
}

func TestStatusReport(t *testing.T) {
	t.Setenv("MYAGENTCFG_DIR", t.TempDir())

	// A source repo, committed so sourceCommit yields a real hash.
	src := makeSourceRepo(t)
	repoRoot := filepath.Dir(src)
	current := gitCommit(t, repoRoot)
	if current == "" {
		t.Skip("git unavailable; cannot exercise flavor drift")
	}

	// An environment with two live symlinks recorded under different kinds.
	comps, _ := scanComponents(src)
	target := filepath.Join(t.TempDir(), ".claude")
	for _, c := range comps {
		if _, err := apply(target, c, resInstall); err != nil {
			t.Fatalf("install %s: %v", c.RelPath, err)
		}
	}
	m := loadManifest(target)
	m.record("skills/grill-me", InstanceRecord{Kind: "symlink", Source: comps[0].Source, InstalledAt: "2026-07-06T10:00:00Z"})
	m.record("commands/deploy.md", InstanceRecord{Kind: "flavored", Source: comps[1].Source, InstalledAt: "2026-07-06T11:00:00Z"})
	if err := m.save(); err != nil {
		t.Fatal(err)
	}

	// Two flavors: one frozen at an old commit (drifted), one at the current.
	seedFlavor(t, "orchestrate-fast", "orchestrate", "stale123")
	seedFlavor(t, "grill-tight", "grill-me", current)

	var buf bytes.Buffer
	writeStatus(&buf, src)
	out := buf.String()

	for _, want := range []string{
		"Environments:",
		envSlug(target),   // the environment label
		target,            // the target path
		"symlink",         // kind label
		"skills/grill-me", // instance rel path
		"flavored",
		"commands/deploy.md",
		"2026-07-06T10:00:00Z", // install time
		"Flavors:",
		"orchestrate-fast",
		"grill-tight",
		"update available", // drifted flavor
		"up to date",       // current flavor
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q\n---\n%s", want, out)
		}
	}

	// The drifted flavor must be flagged, the current one must not be.
	if !strings.Contains(lineFor(out, "orchestrate-fast"), "update available") {
		t.Errorf("stale flavor not flagged as update available:\n%s", out)
	}
	if strings.Contains(lineFor(out, "grill-tight"), "update available") {
		t.Errorf("current flavor wrongly flagged as update available:\n%s", out)
	}
}

func TestStatusNothingInstalled(t *testing.T) {
	t.Setenv("MYAGENTCFG_DIR", t.TempDir())
	src := makeSourceRepo(t)

	var buf bytes.Buffer
	writeStatus(&buf, src)
	if out := buf.String(); !strings.Contains(out, "Nothing installed yet") {
		t.Errorf("empty store should report nothing installed, got:\n%s", out)
	}
}

// lineFor returns the first output line containing substr (for asserting on the
// flag that trails a specific flavor row).
func lineFor(out, substr string) string {
	for _, ln := range strings.Split(out, "\n") {
		if strings.Contains(ln, substr) {
			return ln
		}
	}
	return ""
}
