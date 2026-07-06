# myagent

A warehouse of my Claude Code settings, notes, experiments, and workflows —
plus a TUI to install them into any project or globally.

## Layout

```
.claude/skills/     Claude Code skills (the reusable components)
.claude/agents/     subagents used by the orchestrate pipeline
.claude/commands/   slash-command entrypoints (e.g. /orchestrate)
installer/          Charm/Bubble Tea TUI that symlinks components into place
Makefile            top-level tasks (run / list / build / test)
```

## Skills

| Skill | What it does |
|-------|--------------|
| `grill-me` | Grilling session that stress-tests a plan against the existing domain model, sharpens terminology, and updates docs (`UBIQUITOUS_LANGUAGE.md`, ADRs) inline as decisions crystallise. |
| `compact-smart` | Prepares a session-scoped, coding-aware compaction directive for continuing long-running work across milestones — use before `/compact`. |
| `orchestrate` | Turns the session into a **Manager** (Opus) driving a resumable spec → review → build → review → integrate pipeline across subagents (`architect`, `builder`, `spec-preflight`, `code-preflight`) with optional Codex as an external gating reviewer. Run `/orchestrate`. Runtime state lives under `.orchestrate/` (gitignored). |

## Installer

An interactive TUI that discovers the components in this repo and installs them
via **symlinks** into either the **global** (`~/.claude/`) or a **project**
(`<dir>/.claude/`) namespace. Because it symlinks, this repo stays the single
source of truth — edits propagate to every install instantly.

```bash
make run          # launch the TUI
make list         # print discovered components (no TTY)
make build        # build installer/myagent-install
make test         # run the installer tests
```

Highlights:

- **Global / project toggle**; the project picker has **tab completion**.
- **Idempotent** — already-installed components start checked and tagged
  `● installed`; unchecking one **uninstalls** it (removes the symlink).
- **Per-conflict resolution** (skip / overwrite / backup) when a destination is
  occupied by a real file or a foreign symlink.

See [`installer/README.md`](installer/README.md) for the full flow.
