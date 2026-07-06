# MCP servers

Each `*.json` file in this directory declares one MCP server the installer can
add to a target. The **file name is the server name** (`context7.json` →
`context7`), and the file's contents are the server object exactly as it appears
inside Claude Code's `mcpServers` map.

## Format

**stdio server** (a local command):

```json
{
  "command": "npx",
  "args": ["-y", "@upstash/context7-mcp@latest"],
  "env": {}
}
```

**remote server** (SSE / HTTP):

```json
{
  "type": "sse",
  "url": "https://mcp.example.com/sse"
}
```

The installer copies the object verbatim, so any field Claude Code accepts in an
`mcpServers` entry works here.

## How install works

Unlike skills/commands/agents (which are **symlinked** into `.claude/`), an MCP
server is **merged into Claude Code's MCP config** — it isn't a symlink:

| Install target | Config file written | Claude scope |
|----------------|---------------------|--------------|
| Project (`<dir>/.claude`) | `<dir>/.mcp.json` | project |
| Global (`~/.claude`) | `~/.claude.json` | user |

Installing sets `mcpServers.<name>` in that file (every other key is preserved).
Uninstalling (uncheck an installed server) deletes just that key. If a server
with the same name already exists with a **different** definition, the installer
treats it as a conflict and offers *skip* or *overwrite*.

The installer records each MCP install in its per-environment manifest (kind
`mcp`), reconciled by checking the config file rather than a symlink.

## Adding a server

Drop a new `<name>.json` here and re-run the installer (`make run`); it appears
under **MCP Servers** in the checklist. `README.md` and any non-`.json` file are
ignored by the scanner.

> `context7.json` is a ready-to-use example (Upstash Context7 docs server).
> Keep it, edit it, or delete it.
