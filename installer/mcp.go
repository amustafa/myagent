package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// MCP servers are installed differently from every other component. A skill or
// command is a file/dir symlinked into <target>/.claude/. An MCP server is a
// JSON definition that must be *merged into* Claude Code's MCP config:
//
//   - project target  -> <project-root>/.mcp.json      (Claude's project scope)
//   - global target   -> ~/.claude.json                (Claude's user scope)
//
// In the repo, each server is one file under .claude/mcp/<name>.json whose
// contents are the server object exactly as it appears inside the "mcpServers"
// map (command/args/env for stdio, or type/url for sse|http). The server's key
// is the filename without the .json extension.
//
// So "installing" an MCP server means: set mcpServers[<name>] = <def> in the
// target config file, preserving every other key already there. Uninstalling
// deletes that one key. This is a merge, not a symlink, so it gets its own
// classify/choices/apply that plug into the same destState/resolution model the
// symlink flow uses.

// isMCP reports whether a component is an MCP server definition.
func isMCP(c Component) bool { return c.Type == "mcp" }

// mcpServerName is the server key for a component (its filename without .json).
func mcpServerName(c Component) string {
	return strings.TrimSuffix(c.Name, ".json")
}

// mcpFilePath returns the Claude MCP config file a target installs into.
func mcpFilePath(targetClaude string) string {
	if home, err := os.UserHomeDir(); err == nil && targetClaude == filepath.Join(home, ".claude") {
		return filepath.Join(home, ".claude.json") // user scope
	}
	return filepath.Join(filepath.Dir(targetClaude), ".mcp.json") // project scope
}

// loadMCPDef reads a server definition file (the inner server object).
func loadMCPDef(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	def := map[string]any{}
	if err := json.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return def, nil
}

// readMCPDoc loads the full target config document. A missing or empty file is
// an empty document — every other top-level key it holds is preserved on write.
func readMCPDoc(file string) (map[string]any, error) {
	data, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	doc := map[string]any{}
	if len(bytes.TrimSpace(data)) == 0 {
		return doc, nil
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(file), err)
	}
	return doc, nil
}

// serversOf returns the mcpServers map from a document (creating a fresh one if
// absent or the wrong type).
func serversOf(doc map[string]any) map[string]any {
	if s, ok := doc["mcpServers"].(map[string]any); ok {
		return s
	}
	return map[string]any{}
}

// writeMCPDoc atomically writes the document back, preserving key data.
func writeMCPDoc(file string, doc map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}

// mcpServerPresent reports whether a server (identified by a manifest rel path
// like "mcp/context7.json") is currently in the target's MCP config. Used by
// manifest reconciliation, which can't rely on a symlink existing.
func mcpServerPresent(targetClaude, rel string) bool {
	name := strings.TrimSuffix(filepath.Base(rel), ".json")
	doc, err := readMCPDoc(mcpFilePath(targetClaude))
	if err != nil {
		return false
	}
	_, ok := serversOf(doc)[name]
	return ok
}

// classifyMCP mirrors classifyDest for MCP servers. The returned string is the
// config file involved (shown to the user), analogous to a symlink target.
func classifyMCP(targetClaude string, c Component) (destState, string) {
	file := mcpFilePath(targetClaude)
	def, err := loadMCPDef(c.Source)
	if err != nil {
		return destFree, file // unreadable source def — nothing we can install
	}
	doc, err := readMCPDoc(file)
	if err != nil {
		return destOccupied, file // target file is unparseable — treat as a conflict
	}
	cur, present := serversOf(doc)[mcpServerName(c)]
	switch {
	case !present:
		return destFree, file
	case reflect.DeepEqual(cur, def):
		return destLinkedToUs, file // already installed, identical definition
	default:
		return destOccupied, file // present but a different definition
	}
}

// choicesForMCP is the MCP analogue of choicesFor. There's no "backup" for a
// JSON merge — a differing definition is simply skipped or overwritten.
func choicesForMCP(s destState) []resolution {
	switch s {
	case destFree:
		return []resolution{resInstall}
	case destLinkedToUs:
		return []resolution{resSkip, resRemove}
	default: // destOccupied — present with a different definition
		return []resolution{resSkip, resOverwrite}
	}
}

// applyMCP performs a resolution for an MCP server by editing the target config.
func applyMCP(targetClaude string, c Component, r resolution) (string, error) {
	file := mcpFilePath(targetClaude)
	name := mcpServerName(c)
	switch r {
	case resSkip:
		return "skipped", nil
	case resRemove:
		doc, err := readMCPDoc(file)
		if err != nil {
			return "", err
		}
		servers := serversOf(doc)
		delete(servers, name)
		doc["mcpServers"] = servers
		if err := writeMCPDoc(file, doc); err != nil {
			return "", err
		}
		return "removed from " + filepath.Base(file), nil
	case resInstall, resOverwrite:
		def, err := loadMCPDef(c.Source)
		if err != nil {
			return "", err
		}
		doc, err := readMCPDoc(file)
		if err != nil {
			return "", err
		}
		servers := serversOf(doc)
		servers[name] = def
		doc["mcpServers"] = servers
		if err := writeMCPDoc(file, doc); err != nil {
			return "", err
		}
		if r == resOverwrite {
			return "updated in " + filepath.Base(file), nil
		}
		return "added to " + filepath.Base(file), nil
	}
	return "", fmt.Errorf("unsupported MCP resolution")
}

// ---- kind dispatch ---------------------------------------------------------
// These let the TUI treat MCP and symlink components uniformly: it always calls
// the *Component variant, which routes to the right implementation by kind.

func classifyComponent(targetClaude string, c Component) (destState, string) {
	if isMCP(c) {
		return classifyMCP(targetClaude, c)
	}
	return classifyDest(targetClaude, c)
}

func choicesForComponent(c Component, s destState) []resolution {
	if isMCP(c) {
		return choicesForMCP(s)
	}
	return choicesFor(s)
}

func applyComponent(targetClaude string, c Component, r resolution) (string, error) {
	if isMCP(c) {
		return applyMCP(targetClaude, c, r)
	}
	return apply(targetClaude, c, r)
}
