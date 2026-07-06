package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubTemplate writes a minimal flavorable skill whose install.py echoes the
// received options into the rendered output, so tests can assert on them.
func stubTemplate(t *testing.T) Template {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, flavorSchemaFile), `{
      "flavor":"stub","version":1,
      "options":[{"key":"label","label":"Label","type":"string","default":"hi"}]
    }`)
	mustWrite(t, filepath.Join(dir, flavorRenderFile), `import sys, json, os, argparse
ap = argparse.ArgumentParser(); ap.add_argument("--dest", required=True)
a = ap.parse_args()
opts = json.loads(sys.stdin.read() or "{}")
os.makedirs(a.dest, exist_ok=True)
open(os.path.join(a.dest, "SKILL.md"), "w").write("label=" + str(opts.get("label","")))
`)
	schema, err := parseFlavorSchema(dir)
	if err != nil {
		t.Fatal(err)
	}
	return Template{Name: "stub", Dir: dir, Schema: schema}
}

func TestFlavorLifecycle(t *testing.T) {
	t.Setenv("MYAGENTCFG_DIR", t.TempDir())
	tpl := stubTemplate(t)

	// Create.
	inst, err := createFlavor(tpl, "stub-one", map[string]any{"label": "alpha"}, "commit1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	rendered := filepath.Join(inst.Rendered(), "SKILL.md")
	got, err := os.ReadFile(rendered)
	if err != nil {
		t.Fatalf("render output missing: %v", err)
	}
	if string(got) != "label=alpha" {
		t.Errorf("render did not receive options: %q", got)
	}

	// List + provenance.
	all := listFlavors()
	if len(all) != 1 || all[0].Name != "stub-one" {
		t.Fatalf("listFlavors: %+v", all)
	}
	if all[0].Meta.Commit != "commit1" || all[0].Meta.Skill != "stub" {
		t.Errorf("meta not persisted: %+v", all[0].Meta)
	}
	if all[0].Input["label"] != "alpha" {
		t.Errorf("input not persisted: %+v", all[0].Input)
	}

	// Duplicate name is rejected.
	if _, err := createFlavor(tpl, "stub-one", nil, "commit1"); err == nil {
		t.Error("duplicate flavor name should fail")
	}

	// Drift: same commit => no update; different => update available.
	if all[0].updateAvailable("commit1") {
		t.Error("no drift expected at same commit")
	}
	if !all[0].updateAvailable("commit2") {
		t.Error("drift expected at different commit")
	}

	// Update re-renders and re-stamps the commit.
	upd, err := updateFlavor(all[0], "commit2")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if upd.Meta.Commit != "commit2" || upd.Meta.UpdatedAt == "" {
		t.Errorf("update didn't restamp: %+v", upd.Meta)
	}
	if listFlavors()[0].Meta.Commit != "commit2" {
		t.Error("updated commit not persisted")
	}

	// Delete.
	if err := deleteFlavor("stub-one"); err != nil {
		t.Fatal(err)
	}
	if len(listFlavors()) != 0 {
		t.Error("flavor not deleted")
	}
}

func TestFlavorStampsSkillName(t *testing.T) {
	t.Setenv("MYAGENTCFG_DIR", t.TempDir())
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, flavorSchemaFile), `{"flavor":"base","version":1,"options":[]}`)
	mustWrite(t, filepath.Join(dir, flavorRenderFile), `import argparse, os
ap = argparse.ArgumentParser(); ap.add_argument("--dest", required=True)
a = ap.parse_args()
os.makedirs(a.dest, exist_ok=True)
open(os.path.join(a.dest, "SKILL.md"), "w").write("---\nname: base\ndescription: d\n---\nbody\n")
`)
	schema, err := parseFlavorSchema(dir)
	if err != nil {
		t.Fatal(err)
	}
	tpl := Template{Name: "base", Dir: dir, Schema: schema}

	inst, err := createFlavor(tpl, "base-fast", nil, "c1")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(inst.Rendered(), "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "name: base-fast") {
		t.Errorf("flavor name not stamped into SKILL.md:\n%s", data)
	}
	if strings.Contains(string(data), "name: base\n") {
		t.Errorf("template name still present in SKILL.md:\n%s", data)
	}
}

func TestFlavorAsComponent(t *testing.T) {
	inst := FlavorInstance{Name: "orchestrate-fast", Dir: "/reg/orchestrate-fast"}
	c := inst.asComponent()
	if c.RelPath != filepath.Join("skills", "orchestrate-fast") {
		t.Errorf("relpath: %q", c.RelPath)
	}
	if c.Source != filepath.Join("/reg/orchestrate-fast", "rendered") {
		t.Errorf("source: %q", c.Source)
	}
	if c.Flavor == nil || c.Flavor.Name != "orchestrate-fast" {
		t.Error("flavor pointer not set")
	}
}
