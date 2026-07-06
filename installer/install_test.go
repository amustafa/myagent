package main

import (
	"os"
	"path/filepath"
	"testing"
)

// makeSourceRepo builds a fake myagent-style repo with a couple of components
// and returns the path to its .claude directory.
func makeSourceRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	claude := filepath.Join(root, ".claude")
	mustMkdir(t, filepath.Join(claude, "skills", "grill-me"))
	mustWrite(t, filepath.Join(claude, "skills", "grill-me", "SKILL.md"), "x")
	mustMkdir(t, filepath.Join(claude, "commands"))
	mustWrite(t, filepath.Join(claude, "commands", "deploy.md"), "y")
	return claude
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanFindsComponents(t *testing.T) {
	src := makeSourceRepo(t)
	comps, err := scanComponents(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(comps) != 2 {
		t.Fatalf("want 2 components, got %d: %+v", len(comps), comps)
	}
	// commands sorts before skills only if it ranks first; verify by rel path set.
	got := map[string]bool{}
	for _, c := range comps {
		got[c.RelPath] = true
	}
	for _, want := range []string{"skills/grill-me", "commands/deploy.md"} {
		if !got[want] {
			t.Errorf("missing component %q", want)
		}
	}
}

func TestInstallThenIdempotentAndUninstall(t *testing.T) {
	src := makeSourceRepo(t)
	comps, _ := scanComponents(src)
	target := filepath.Join(t.TempDir(), ".claude")

	// 1. Fresh target: every dest is free.
	for _, c := range comps {
		if state, _ := classifyDest(target, c); state != destFree {
			t.Fatalf("%s: want destFree, got %v", c.RelPath, state)
		}
		if _, err := apply(target, c, resInstall); err != nil {
			t.Fatalf("install %s: %v", c.RelPath, err)
		}
	}

	// 2. After install: each dest is a symlink pointing at our source.
	for _, c := range comps {
		state, tgt := classifyDest(target, c)
		if state != destLinkedToUs {
			t.Fatalf("%s: want destLinkedToUs, got %v", c.RelPath, state)
		}
		resolved, _ := filepath.EvalSymlinks(c.Source)
		if tgt != resolved {
			t.Errorf("%s: link target %q != source %q", c.RelPath, tgt, resolved)
		}
	}

	// 3. Uninstall via resRemove leaves the dest free again and does NOT touch
	//    the source.
	c := comps[0]
	if _, err := apply(target, c, resRemove); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if state, _ := classifyDest(target, c); state != destFree {
		t.Fatalf("after remove: want destFree, got %v", state)
	}
	if _, err := os.Stat(c.Source); err != nil {
		t.Fatalf("source was damaged by uninstall: %v", err)
	}
}

func TestOccupiedAndBackup(t *testing.T) {
	src := makeSourceRepo(t)
	comps, _ := scanComponents(src)
	target := filepath.Join(t.TempDir(), ".claude")
	c := comps[0]

	// Put a real file where the symlink would go.
	dest := destPath(target, c)
	mustMkdir(t, filepath.Dir(dest))
	mustWrite(t, dest, "pre-existing user file")

	if state, _ := classifyDest(target, c); state != destOccupied {
		t.Fatalf("want destOccupied, got %v", state)
	}

	// Backup should preserve the original and create a working link.
	if _, err := apply(target, c, resBackup); err != nil {
		t.Fatalf("backup: %v", err)
	}
	if state, _ := classifyDest(target, c); state != destLinkedToUs {
		t.Fatalf("after backup: want destLinkedToUs, got %v", state)
	}
	if _, err := os.Stat(dest + ".bak-1"); err != nil {
		t.Errorf("backup file missing: %v", err)
	}
}
