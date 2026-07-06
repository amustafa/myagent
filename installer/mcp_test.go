package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// makeMCPSource builds a repo whose .claude/mcp holds one server def plus a
// README (which must be ignored), and returns the .claude path.
func makeMCPSource(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	claude := filepath.Join(root, ".claude")
	mustMkdir(t, filepath.Join(claude, "mcp"))
	mustWrite(t, filepath.Join(claude, "mcp", "context7.json"),
		`{"command":"npx","args":["-y","@upstash/context7-mcp@latest"]}`)
	mustWrite(t, filepath.Join(claude, "mcp", "README.md"), "docs")
	return claude
}

func mcpComponent(t *testing.T, sourceClaude string) Component {
	t.Helper()
	comps, err := scanComponents(sourceClaude)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range comps {
		if isMCP(c) {
			return c
		}
	}
	t.Fatal("no MCP component scanned")
	return Component{}
}

func TestMCPScanFiltersNonJSON(t *testing.T) {
	src := makeMCPSource(t)
	comps, _ := scanComponents(src)
	var mcp []Component
	for _, c := range comps {
		if isMCP(c) {
			mcp = append(mcp, c)
		}
	}
	if len(mcp) != 1 || mcp[0].Name != "context7.json" {
		t.Fatalf("want exactly context7.json, got %+v", mcp)
	}
	if mcpServerName(mcp[0]) != "context7" {
		t.Errorf("server name: got %q", mcpServerName(mcp[0]))
	}
}

func readServersMap(t *testing.T, file string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	doc := map[string]any{}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return serversOf(doc)
}

func TestMCPInstallRemoveRoundTrip(t *testing.T) {
	src := makeMCPSource(t)
	c := mcpComponent(t, src)
	target := filepath.Join(t.TempDir(), ".claude")
	file := mcpFilePath(target)

	// Pre-seed the project .mcp.json with unrelated content to prove it survives.
	mustWrite(t, file, `{"someOtherKey":"keep","mcpServers":{"existing":{"command":"foo"}}}`)

	// Free before install.
	if s, _ := classifyMCP(target, c); s != destFree {
		t.Fatalf("before install: want destFree, got %v", s)
	}
	if _, err := applyMCP(target, c, resInstall); err != nil {
		t.Fatal(err)
	}

	// Installed + neighbors preserved.
	if s, _ := classifyMCP(target, c); s != destLinkedToUs {
		t.Fatalf("after install: want destLinkedToUs, got %v", s)
	}
	servers := readServersMap(t, file)
	if _, ok := servers["context7"]; !ok {
		t.Error("context7 not written")
	}
	if _, ok := servers["existing"]; !ok {
		t.Error("existing server was clobbered")
	}
	var doc map[string]any
	data, _ := os.ReadFile(file)
	_ = json.Unmarshal(data, &doc)
	if doc["someOtherKey"] != "keep" {
		t.Error("unrelated top-level key was lost")
	}

	// Remove leaves the file valid and the server gone, neighbors intact.
	if _, err := applyMCP(target, c, resRemove); err != nil {
		t.Fatal(err)
	}
	if s, _ := classifyMCP(target, c); s != destFree {
		t.Fatalf("after remove: want destFree, got %v", s)
	}
	if _, ok := readServersMap(t, file)["existing"]; !ok {
		t.Error("remove clobbered a neighbor")
	}
}

func TestMCPConflictDifferentDefinition(t *testing.T) {
	src := makeMCPSource(t)
	c := mcpComponent(t, src)
	target := filepath.Join(t.TempDir(), ".claude")
	file := mcpFilePath(target)

	// Same server name, different definition => conflict.
	mustWrite(t, file, `{"mcpServers":{"context7":{"command":"OTHER"}}}`)
	if s, _ := classifyMCP(target, c); s != destOccupied {
		t.Fatalf("want destOccupied, got %v", s)
	}
	got := choicesForMCP(destOccupied)
	if len(got) != 2 || got[0] != resSkip || got[1] != resOverwrite {
		t.Fatalf("conflict choices: got %v", got)
	}

	// Overwrite replaces with our definition.
	if _, err := applyMCP(target, c, resOverwrite); err != nil {
		t.Fatal(err)
	}
	if s, _ := classifyMCP(target, c); s != destLinkedToUs {
		t.Fatalf("after overwrite: want destLinkedToUs, got %v", s)
	}
}

func TestMCPFilePath(t *testing.T) {
	home, _ := os.UserHomeDir()
	if got := mcpFilePath(filepath.Join(home, ".claude")); got != filepath.Join(home, ".claude.json") {
		t.Errorf("global: got %q", got)
	}
	proj := "/home/x/projY/.claude"
	if got := mcpFilePath(proj); got != "/home/x/projY/.mcp.json" {
		t.Errorf("project: got %q", got)
	}
}
