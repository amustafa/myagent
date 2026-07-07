package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// manifestWith builds an in-memory manifest whose recorded instances are the
// given relative paths, rooted at target.
func manifestWith(target string, rels ...string) *Manifest {
	m := &Manifest{Target: target, Instances: map[string]InstanceRecord{}}
	for _, r := range rels {
		m.Instances[r] = InstanceRecord{Kind: "symlink"}
	}
	return m
}

func TestSyncPromptsIndex_WritesImportsForPromptsOnly(t *testing.T) {
	target := t.TempDir()
	m := manifestWith(target,
		"prompts/worktrees.md",
		"prompts/model-selection.md",
		"skills/grill-me", // must be ignored
		"agents/architect.md",
	)

	n, err := syncPromptsIndex(target, m)
	if err != nil {
		t.Fatalf("syncPromptsIndex: %v", err)
	}
	if n != 2 {
		t.Fatalf("linked count = %d, want 2", n)
	}

	data, err := os.ReadFile(filepath.Join(target, "prompts", "_index.md"))
	if err != nil {
		t.Fatalf("reading index: %v", err)
	}
	got := string(data)

	// Imports are sibling-relative and sorted; non-prompt components excluded.
	for _, want := range []string{"@model-selection.md", "@worktrees.md"} {
		if !strings.Contains(got, want+"\n") {
			t.Errorf("index missing import %q; got:\n%s", want, got)
		}
	}
	for _, bad := range []string{"@prompts/", "grill-me", "architect"} {
		if strings.Contains(got, bad) {
			t.Errorf("index should not contain %q; got:\n%s", bad, got)
		}
	}
	// Sorted order: model-selection before worktrees.
	if strings.Index(got, "@model-selection.md") > strings.Index(got, "@worktrees.md") {
		t.Errorf("imports not sorted; got:\n%s", got)
	}
}

func TestSyncPromptsIndex_RemovesFileWhenNoPromptsRemain(t *testing.T) {
	target := t.TempDir()
	dest := filepath.Join(target, "prompts", "_index.md")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("@stale.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Manifest has no prompt entries (only a skill) — index should be removed.
	m := manifestWith(target, "skills/grill-me")
	n, err := syncPromptsIndex(target, m)
	if err != nil {
		t.Fatalf("syncPromptsIndex: %v", err)
	}
	if n != 0 {
		t.Fatalf("linked count = %d, want 0", n)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("expected index removed, stat err = %v", err)
	}
}

func TestSyncPromptsIndex_EmptyIsNoOpNotError(t *testing.T) {
	target := t.TempDir()
	// No index present, no prompts recorded: must not error, must not create.
	n, err := syncPromptsIndex(target, manifestWith(target))
	if err != nil {
		t.Fatalf("syncPromptsIndex: %v", err)
	}
	if n != 0 {
		t.Fatalf("linked count = %d, want 0", n)
	}
	if _, err := os.Stat(filepath.Join(target, "prompts", "_index.md")); !os.IsNotExist(err) {
		t.Errorf("index should not have been created; stat err = %v", err)
	}
}

// The index file itself must never appear as one of its own imports.
func TestSyncPromptsIndex_ExcludesItself(t *testing.T) {
	target := t.TempDir()
	m := manifestWith(target, "prompts/worktrees.md", promptsIndexRel)
	n, err := syncPromptsIndex(target, m)
	if err != nil {
		t.Fatalf("syncPromptsIndex: %v", err)
	}
	if n != 1 {
		t.Fatalf("linked count = %d, want 1", n)
	}
	data, _ := os.ReadFile(filepath.Join(target, "prompts", "_index.md"))
	if strings.Contains(string(data), "@_index.md") {
		t.Errorf("index imported itself; got:\n%s", data)
	}
}
