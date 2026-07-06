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

## Flavors

A skill can be **flavored** — configured at install time instead of shipping one
fixed behavior. A skill is flavorable when it carries two extra files:

- `flavor.json` — the option schema (what's configurable and how).
- `install.py` — a renderer the installer runs to materialize a configured copy.

Flavorable skills don't appear in the plain list; they're **templates** you turn
into named flavors via `＋ Add new flavor`:

1. Pick a flavorable skill → fill the form → name the flavor.
2. The installer runs `install.py` (option values as JSON on stdin, `--dest` a
   render directory) and stores the result globally under
   `${MYAGENTCFG_DIR}/flavors/<name>/` (`input.json` + `meta.json` + `rendered/`).
3. The flavor then shows up in the **Flavors** section of the list and is
   installed/uninstalled into the current environment like any other component
   (a symlink from `rendered/` into `<target>/.claude/skills/<name>/`).

Per-flavor row actions: `space` install/uninstall · `u` update · `d` delete.

### Option types

`string`, `number` (with `min`/`max`/`integer`), `bool`, `path` (tab-completes),
`enum-one`, `enum-or-custom` (pick a known value or type your own), `enum-set`
(unordered multi), `enum-list` (ordered multi — reorder with `<`/`>`). Any option
can carry `showIf` to appear only when another option has a given value, plus
`required`/`regex` validation.

### Updating on drift

A flavor is *frozen* at the source commit it was rendered from (basic skills, by
contrast, symlink live source and are always current). When the source repo moves
to a new commit, the flavor row shows `update available`; `u` re-runs `install.py`
with the saved `input.json` and regenerates `rendered/` in place — so every
environment symlinking it picks up the change with no re-install.
