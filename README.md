# myagent

A warehouse of my Claude Code settings, skills, and workflows — plus a TUI that
installs them into any project or globally via symlinks.

## Layout

```
.claude/skills/     Claude Code skills (some flavorable — configured at install time)
.claude/agents/     subagents used by the orchestrate pipeline
.claude/prompts/    prompt frameworks, indexed into a `_index.md` you import from CLAUDE.md
.claude/mcp/        MCP server definitions (merged into Claude's MCP config)
installer/          Charm/Bubble Tea TUI that installs the above
Makefile            top-level tasks (run / list / status / install / test / ci)
```

## Skills

| Skill | What it does |
|-------|--------------|
| `grill-me` | Grilling session that stress-tests a plan against the existing domain model, sharpens terminology, and updates docs (`UBIQUITOUS_LANGUAGE.md`, ADRs) inline as decisions crystallise. |
| `compact-smart` | Prepares a session-scoped, coding-aware compaction directive for continuing long-running work across milestones — use before `/compact`. |
| `codex-computer-use` | Delegates computer-use / runtime operations (launching apps, booting simulators, screenshots, runtime inspection) to gpt-5.5 via the Codex CLI, keeping screenshot blobs and build spew out of your context. |
| `orchestrate` | Turns the session into a **Manager** (Opus) driving a resumable spec → review → build → review → integrate pipeline across subagents (`architect`, `builder`, `spec-preflight`, `code-preflight`) with optional Codex as an external gating reviewer. Run `/orchestrate`. **Flavorable** — pick model tiers, codex on/off, etc. at install time. |

## Installer

An interactive TUI that installs this repo's components into either the
**global** (`~/.claude/`) or a **project** (`<dir>/.claude/`) namespace. Skills,
commands, and agents are **symlinked** (so the repo stays the single source of
truth); MCP servers are **merged** into Claude's MCP config.

```bash
make run          # launch the TUI
make install      # put a `myagent` binary on your PATH (runs from anywhere)
make list         # print discovered components (no TUI)
make status       # report what's installed across environments
make test         # run the installer's tests
make ci           # what CI runs: gofmt check + build + vet + test
```

`make install` bakes this repo in as the default source, so a global `myagent`
binary always installs *from here* regardless of the working directory (drop
`~/.local/bin` — or your `BINDIR` — on your `PATH`).

Highlights:

- **Global / project toggle**; the project picker has **tab completion**.
- **Idempotent** — already-installed components start checked; unchecking an
  installed item uninstalls it. Every install is recorded in a per-environment
  manifest under `~/.config/myagentcfg/`.
- **Per-conflict resolution** — skip / overwrite / backup when a destination is
  already occupied.
- **Flavoring** — a skill (or MCP server) can ship `flavor.json` + `install.py`
  to be *configured* at install time. Create named flavors via `＋ Add new
  flavor`, then install / edit / update-on-drift / delete them per environment.

See [`installer/README.md`](installer/README.md) for the full flow, option
types, and the flavoring/MCP details.

## CI

Every push to `main` and every PR runs `.github/workflows/ci.yml` (gofmt check,
`go build`, `go vet`, `go test`) — the same thing `make ci` runs locally.
