# myagent installer

A Charm/Bubble Tea TUI that installs this repo's Claude Code components
(skills, commands, agents, …) into the **global** (`~/.claude/`) or a
**project** (`<dir>/.claude/`) namespace using **symlinks** — so the repo stays
the single source of truth and edits propagate everywhere instantly.

## Run

```bash
cd installer
go run .            # launch the TUI
# or build a binary:
go build -o myagent-install . && ./myagent-install
```

It auto-detects the source repo by walking up from the current directory to the
nearest `.claude/`. Override with `-source /path/to/repo`.

```bash
go run . -list      # print discovered components, no TUI
```

## Flow

1. **Namespace** — toggle Global vs Project (`←/→`/`tab`, `enter`).
2. **Project dir** (project only) — type a path with `Tab` completion.
3. **Components** — checklist grouped by type. Already-installed items start
   **checked** and tagged `● installed`.
   - `space` toggle · `a` all/none · `enter` apply.
   - **Check** an item to install it; **uncheck** an installed item to
     **uninstall** it (removes the symlink).
4. **Conflicts** — if a real file or a foreign symlink already occupies a
   destination, you're prompted per item: **skip / overwrite / backup**.
5. **Done** — per-component results.

## Idempotent

Re-running is safe: components already symlinked to this repo are detected,
shown as installed, and left untouched unless you explicitly uncheck them.

## MCP servers

MCP servers are a different *kind* of component. A skill/command/agent is a file
or directory **symlinked** into `.claude/`; an MCP server is a JSON definition
**merged into** Claude Code's MCP config.

Declare each server as one file under `.claude/mcp/<name>.json` (the file name is
the server key; the contents are the server object as it appears in
`mcpServers`). Installing writes it to:

| Target | Config file | Claude scope |
|--------|-------------|--------------|
| Project | `<project>/.mcp.json` | project |
| Global | `~/.claude.json` | user |

Every other key in that file is preserved. Uninstalling (uncheck an installed
server) deletes just that one `mcpServers` entry; a same-name/different-definition
collision prompts *skip* or *overwrite*. See `.claude/mcp/README.md` for the
definition format. (`claude mcp add` remains a fine manual alternative; this just
version-controls your servers alongside the rest of the pack.)

## Install state

Every apply is recorded in a per-environment manifest under
`${MYAGENTCFG_DIR:-~/.config/myagentcfg}/environments/<env>/installed.json`,
where an **environment** is one install target — `global` for `~/.claude`, or a
path-slug for a project. Each entry records the component's source, install
time, and the source repo's commit at install time.

The manifest is a *record*, not the source of truth: it's reconciled against the
filesystem on load, so a symlink you delete by hand is dropped from the manifest
rather than lingering as a lie. Set `MYAGENTCFG_DIR` to relocate the store.
