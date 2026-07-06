package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// --- edit flavor -------------------------------------------------------------

func TestEditFlavorRerendersWithNewInput(t *testing.T) {
	t.Setenv("MYAGENTCFG_DIR", t.TempDir())
	tpl := stubTemplate(t) // echoes options into rendered SKILL.md (registry_test.go)

	inst, err := createFlavor(tpl, "edit-me", map[string]any{"label": "one"}, "c1")
	if err != nil {
		t.Fatal(err)
	}

	ed, err := editFlavor(inst, tpl.Dir, map[string]any{"label": "two"}, "c2")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(ed.Rendered(), "SKILL.md"))
	if string(got) != "label=two" {
		t.Errorf("edit didn't re-render with new input: %q", got)
	}
	if ed.Meta.Commit != "c2" || ed.Meta.UpdatedAt == "" {
		t.Errorf("edit didn't restamp: %+v", ed.Meta)
	}
	if listFlavors()[0].Input["label"] != "two" {
		t.Error("new input not persisted to input.json")
	}
}

// --- cross-environment usage -------------------------------------------------

func TestEnvironmentsUsingFlavor(t *testing.T) {
	t.Setenv("MYAGENTCFG_DIR", t.TempDir())
	targetA := filepath.Join(t.TempDir(), "projA", ".claude")
	targetB := filepath.Join(t.TempDir(), "projB", ".claude")

	for _, target := range []string{targetA, targetB} {
		m := loadManifest(target)
		m.record("skills/shared", InstanceRecord{
			Kind: "flavored", Source: "/x", InstalledAt: nowStamp(),
			Options: map[string]any{"flavor": "shared"},
		})
		// reconcile prunes entries without a live symlink, so drop a real link in.
		mustMkdir(t, filepath.Dir(filepath.Join(target, "skills", "shared")))
		if err := os.Symlink(t.TempDir(), filepath.Join(target, "skills", "shared")); err != nil {
			t.Fatal(err)
		}
		if err := m.save(); err != nil {
			t.Fatal(err)
		}
	}

	users := environmentsUsingFlavor("shared")
	if len(users) != 2 {
		t.Fatalf("want 2 environments using the flavor, got %d: %v", len(users), users)
	}
	if len(environmentsUsingFlavor("nobody")) != 0 {
		t.Error("unknown flavor should have no users")
	}
}

// --- MCP flavoring -----------------------------------------------------------

// mcpStubTemplate writes a flavorable MCP template whose install.py renders a
// server.json from the chosen package option.
func mcpStubTemplate(t *testing.T) Template {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, flavorSchemaFile), `{
      "flavor":"mcpstub","version":1,
      "options":[{"key":"pkg","label":"Package","type":"string","default":"default-pkg"}]
    }`)
	mustWrite(t, filepath.Join(dir, flavorRenderFile), `import sys, json, os, argparse
ap = argparse.ArgumentParser(); ap.add_argument("--dest", required=True)
a = ap.parse_args()
opts = json.loads(sys.stdin.read() or "{}")
os.makedirs(a.dest, exist_ok=True)
server = {"command": "npx", "args": ["-y", opts.get("pkg", "x")]}
open(os.path.join(a.dest, "server.json"), "w").write(json.dumps(server))
`)
	schema, err := parseFlavorSchema(dir)
	if err != nil {
		t.Fatal(err)
	}
	return Template{Name: "mcpstub", Dir: dir, Schema: schema, Target: "mcp"}
}

func TestMCPFlavorInstallsViaMergePath(t *testing.T) {
	t.Setenv("MYAGENTCFG_DIR", t.TempDir())
	tpl := mcpStubTemplate(t)

	inst, err := createFlavor(tpl, "ctx-fast", map[string]any{"pkg": "@my/mcp"}, "c1")
	if err != nil {
		t.Fatal(err)
	}
	if inst.Meta.Target != "mcp" {
		t.Fatalf("meta target: %q", inst.Meta.Target)
	}

	// The render produced a single server.json holding the chosen package.
	raw, err := os.ReadFile(filepath.Join(inst.Rendered(), mcpRenderedFile))
	if err != nil {
		t.Fatalf("server.json not rendered: %v", err)
	}
	var def map[string]any
	if err := json.Unmarshal(raw, &def); err != nil {
		t.Fatal(err)
	}

	// It presents as an MCP component, not a skill symlink.
	c := inst.asComponent()
	if !isMCP(c) {
		t.Fatal("flavored MCP should present as an MCP component")
	}
	if c.RelPath != filepath.Join("mcp", "ctx-fast.json") || mcpServerName(c) != "ctx-fast" {
		t.Errorf("mcp component identity wrong: rel=%q name=%q", c.RelPath, mcpServerName(c))
	}

	// Installing routes through the MCP merge path and lands the server in config.
	target := filepath.Join(t.TempDir(), ".claude")
	if s, _ := classifyComponent(target, c); s != destFree {
		t.Fatalf("before install: want destFree, got %v", s)
	}
	if _, err := applyComponent(target, c, resInstall); err != nil {
		t.Fatal(err)
	}
	if s, _ := classifyComponent(target, c); s != destLinkedToUs {
		t.Fatalf("after install: want destLinkedToUs, got %v", s)
	}
	servers := readServersMap(t, mcpFilePath(target)) // helper from mcp_test.go
	if _, ok := servers["ctx-fast"]; !ok {
		t.Error("flavored MCP server not written into config")
	}
}

func TestScanTemplatesFindsMCPTemplate(t *testing.T) {
	root := t.TempDir()
	claude := filepath.Join(root, ".claude")
	dir := filepath.Join(claude, "mcp", "myserver")
	mustMkdir(t, dir)
	mustWrite(t, filepath.Join(dir, flavorSchemaFile), `{"flavor":"x","version":1,"options":[]}`)
	mustWrite(t, filepath.Join(dir, flavorRenderFile), "print('x')")

	tpls, err := scanTemplates(claude)
	if err != nil {
		t.Fatal(err)
	}
	if len(tpls) != 1 || tpls[0].Name != "myserver" || tpls[0].Target != "mcp" {
		t.Fatalf("mcp template not scanned with target: %+v", tpls)
	}
}
