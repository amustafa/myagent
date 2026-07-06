# Stage 2 design — flavoring presentation & MCP installs

Refines the Stage 2 plan. Stage 1 (per-environment `installed.json` manifest,
commit capture, reconcile) is merged and is the foundation here.

## 1. Core reconception: a flavor is a first-class, generated component

The earlier plan flavored a skill *inline* during install. This design
**decouples flavor creation from installation**:

1. **Create a flavor** (separate flow) — pick a flavorable skill, fill the
   flavor form, name the instance. This *generates* the flavor: runs the skill's
   `install.py` to render the resolved skill, and records it in the config. It
   does **not** install into any environment yet.
2. The generated flavor then **appears on the main component list** next to the
   basic skills, and from there is checked/installed into the current
   environment via the ordinary symlink flow — "just like any other skill."
3. Generated flavors can be **deleted**.
4. Generated flavors can be **updated** on commit drift (see §3).

A flavor is therefore a reusable, named recipe you create once and can install
into many environments.

## 2. On-disk layout

Flavor **definitions are global** (a recipe is environment-independent);
**installs stay per-environment** (Stage 1).

```
${MYAGENTCFG_DIR:-~/.config/myagentcfg}/
├── flavors/                          # global flavor registry
│   └── <instance>/                   # e.g. orchestrate-fast
│       ├── input.json                # saved option values (re-render source of truth)
│       ├── meta.json                 # { skill, source, commit, created_at, updated_at }
│       └── rendered/                 # output of install.py (the resolved skill)
└── environments/<env>/installed.json # Stage 1 manifest; flavored entries point at a flavor instance
```

- **Render-into-registry + symlink-into-target** (the placement Stage 1 chose)
  is what makes updates transparent: an environment symlinks
  `…/flavors/<instance>/rendered/` into `<target>/.claude/skills/<instance>/`.
  Regenerating `rendered/` in place updates every environment that links it —
  no per-environment re-install needed.
- Manifest entry for an installed flavor:
  `kind: "flavored", flavor: "<instance>", source, commit`.

## 3. The update flag (per directive)

Basic skills are symlinked to *live* source, so they're always current — commit
drift is meaningless for them. The update flag applies **only to generated
flavors**, which are frozen at the commit they were rendered from.

Mechanics:
1. At startup, compute the current source commit (`git -C <sourceRepo> rev-parse
   --short HEAD`, already in `state.go: sourceCommit`).
2. For each flavor in the registry, compare `meta.commit` to the current source
   commit.
3. If they differ, mark that flavor **`update available`** — offered on an
   **individual (per-flavor) basis**, never bulk.
4. **Update action** = re-run the skill's `install.py` with the flavor's saved
   `input.json`, regenerate `rendered/`, and set `meta.commit` = current +
   `meta.updated_at`. Because environments symlink `rendered/`, they pick up the
   change automatically.

Update never changes the user's choices — it re-renders the *same input* against
new skill code. (Re-opening the form to change choices is a separate "edit
flavor" action, optional for v1.)

## 4. TUI presentation

### Main list — two sections

```
Skills
  [x] grill-me           ● installed
  [ ] compact-smart
Flavors
  [ ] orchestrate-fast   flavored          update available
  [x] orchestrate-safe   flavored  ● installed
  ＋ Add new flavor
```

- **Skills/Commands/Agents**: basic components, existing symlink flow.
- **Flavors**: generated instances. Each is a normal installable row (checkbox →
  symlink flow), plus per-row affordances:
  - `space` — install / uninstall into the current environment (symlink flow).
  - `u` — update (only when `update available`); re-renders from saved input.
  - `d` — delete the generated flavor (confirm) → removes
    `flavors/<instance>/` and uninstalls any symlink to it from the current
    environment (and warns if other environments still link it).
- **`＋ Add new flavor`** — an action row that opens the creation flow.

### "Add new flavor" flow

1. **Pick a flavorable skill** — those shipping both `flavor.json` (schema) and
   `install.py` (renderer).
2. **Flavor form** — one widget per option type (the 6 core types + `path`,
   `enum-or-custom`, `show-if` visibility, and validation). Ordered-list options
   support move-up/down.
3. **Name the instance** — default = skill name; must be unique in the registry;
   installing a second flavor of the same skill forces a distinct name
   (e.g. `orchestrate-fast`).
4. **Generate** — run `install.py` (options on stdin, `--dest
   flavors/<instance>/rendered`), write `input.json` + `meta.json`, return to
   the main list with the new flavor present but not yet installed.

Deleting is registry-scoped; installing/uninstalling is environment-scoped — the
two are independent, exactly as the directive asks.

## 5. MCP installations (suggested — nothing exists here yet)

Claude Code already has first-class MCP support, so lean on it rather than
inventing a parallel mechanism. MCP servers don't fit the symlink-a-directory
model (they're config entries, not files that live in a namespace), so they get
a distinct **merge** flow.

### A location to collect them

Add `.claude/mcp/` to the repo — one self-contained JSON per server:

```
.claude/mcp/github.json
.claude/mcp/linear.json
```

```jsonc
// .claude/mcp/github.json
{
  "name": "github",
  "config": {
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-github"],
    "env": { "GITHUB_TOKEN": "${GITHUB_TOKEN}" }   // env-var placeholder, never a literal secret
  }
}
```

The installer scans `.claude/mcp/*.json` as a new component **type** (`mcp`),
listed in its own section like any other.

### How install works (merge, not symlink)

- **Project target** → merge the selected `config` blocks into
  `<target>/.mcp.json` under `mcpServers` (create the file if absent). `.mcp.json`
  is the standard project-scoped file Claude Code reads and is safe to commit.
- **Global/user target** → shell out to Claude's own CLI:
  `claude mcp add-json <name> '<config>' -s user` per selected server. This is
  "the easiest thing" the user referenced and keeps user-scope config in the
  place Claude expects (`~/.claude.json`).
- **Manifest**: record as `kind: "mcp"`, with the server name and the target
  file / scope, so uninstall is a clean removal of that `mcpServers` key (or
  `claude mcp remove <name>`), and idempotency is a name-presence check.

### Secrets

Never store tokens in the repo. Entries reference `${ENV_VAR}` placeholders,
which Claude Code expands from the environment at load. (This is the same
"secret = env-var reference" idea noted for flavor option types.)

### Optional: MCP entries can be flavored too

Because MCP entries are just JSON with variable bits (which env var, which args,
which model), they can carry a `flavor.json` + `install.py` and go through the
exact same flavor flow — e.g. choosing the token env-var name or a read-only vs
read-write arg set at create time. Not required for v1, but the mechanism
composes for free.

## 6. Build order within Stage 2 (suggested)

1. `flavor.json` schema types + parser + validation.
2. Flavor registry (`flavors/<instance>/`, input/meta/rendered) + `install.py`
   contract + render-and-symlink.
3. Flavor form sub-UI (widgets per type).
4. Main-list two-section presentation + create/install/update/delete affordances
   + the update-flag drift check.
5. `orchestrate` as the first flavored skill (`flavor.json`, `install.py`,
   `orch.py` seed for the 2-tier ordered model lists).
6. MCP: `.claude/mcp/` scan + merge flow + manifest `kind: "mcp"`.
```
