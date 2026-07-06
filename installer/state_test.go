package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnvSlug(t *testing.T) {
	home, _ := os.UserHomeDir()
	if got := envSlug(filepath.Join(home, ".claude")); got != "global" {
		t.Errorf("global target: want \"global\", got %q", got)
	}
	got := envSlug("/home/amustafa/projX/.claude")
	if got != "home_amustafa_projX" {
		t.Errorf("project slug: got %q", got)
	}
}

func TestManifestRecordSaveLoad(t *testing.T) {
	// Redirect the store into a temp dir so we don't touch ~/.config.
	t.Setenv("MYAGENTCFG_DIR", t.TempDir())

	// A real target with a live symlink so reconcile keeps the record.
	src := makeSourceRepo(t)
	comps, _ := scanComponents(src)
	target := filepath.Join(t.TempDir(), ".claude")
	c := comps[0]
	if _, err := apply(target, c, resInstall); err != nil {
		t.Fatal(err)
	}

	m := loadManifest(target)
	m.record(c.RelPath, InstanceRecord{Kind: "symlink", Source: c.Source, InstalledAt: nowStamp(), Commit: "abc1234"})
	if err := m.save(); err != nil {
		t.Fatal(err)
	}

	// Reload: the record survives because the symlink still exists.
	m2 := loadManifest(target)
	rec, ok := m2.Instances[c.RelPath]
	if !ok {
		t.Fatalf("record for %s not persisted", c.RelPath)
	}
	if rec.Commit != "abc1234" || rec.Kind != "symlink" {
		t.Errorf("record round-trip mismatch: %+v", rec)
	}
	if m2.Target != target {
		t.Errorf("target not persisted: %q", m2.Target)
	}
}

func TestManifestReconcilePrunesMissing(t *testing.T) {
	t.Setenv("MYAGENTCFG_DIR", t.TempDir())
	target := filepath.Join(t.TempDir(), ".claude")

	m := loadManifest(target)
	// Record an instance whose symlink was never created (or was deleted).
	m.record("skills/ghost", InstanceRecord{Kind: "symlink", Source: "/nowhere", InstalledAt: nowStamp()})
	if err := m.save(); err != nil {
		t.Fatal(err)
	}

	// Loading reconciles against the filesystem — the dangling record is dropped.
	m2 := loadManifest(target)
	if _, ok := m2.Instances["skills/ghost"]; ok {
		t.Errorf("reconcile should have pruned the missing-symlink record")
	}
}
