# State model & the `orch.py` helper

All state lives under `.orchestrate/` in the repo root. It persists across
sessions and Claude-account switches — this is what makes the pipeline
resumable. Never keep the source of truth only in your context.

## On-disk layout

```
.orchestrate/
├── state.json                 # source of truth (managed by orch.py)
├── STATUS.md                  # human-readable mirror (regenerated on every write)
├── config.json                # (inside state.json) knobs — see below
├── memory.md                  # long-term project memory (you append to this)
├── backlog.md                 # backlog / follow-ups (you append to this)
└── workstreams/
    └── ws-001-add-oauth-login/
        ├── spec.md            # the living spec (Architect owns the content)
        ├── notes.md           # scratch: decisions, open questions
        └── reviews/
            ├── codex-r1.md            # raw Codex output, round 1
            ├── consolidated-r1.md     # your merged/triaged findings, round 1
            ├── codex-r2.md
            └── ...
```

## Phases

| Phase | Meaning | Reference |
|-------|---------|-----------|
| `spec` | Architect is writing / about to write the spec | spec-phase.md |
| `spec_review` | in the spec review loop (preflight + Codex) | spec-phase.md |
| `awaiting_approval` | spec is clean; waiting for human approval to build | spec-phase.md |
| `build` | Builder is implementing / about to | build-phase.md |
| `build_review` | in the code review loop (preflight + Codex) | build-phase.md |
| `integrate` | merging, testing, updating trackers | integration.md |
| `done` | shipped and recorded | — |
| `blocked` | stuck; needs the human | surface it |
| `archived` | shelved, not counted as in-flight | — |

`status` is orthogonal: `in_progress`, `waiting_user`, `blocked`, `done`.
`round` counts revision cycles within the current phase.

## Running the helper

The script is at `<this-skill-dir>/scripts/orch.py`. Always run it from (or
point it at) the repo root:

```bash
# If the repo root is the current directory:
python3 /path/to/.claude/skills/orchestrate/scripts/orch.py <cmd> ...

# If not, set the root explicitly:
ORCH_ROOT=/abs/path/to/repo python3 .../orch.py <cmd> ...
```

Resolve the script path once at session start (it lives beside SKILL.md) and
reuse it. Consider exporting a shell var in your first bash call, e.g.
`ORCH="python3 /abs/.claude/skills/orchestrate/scripts/orch.py"`.

## Command reference

| Command | Effect |
|---------|--------|
| `orch.py init` | create `.orchestrate/` scaffold + memory/backlog files (idempotent) |
| `orch.py new "<title>"` | create a workstream; prints its id (e.g. `ws-002-fix-race`) |
| `orch.py list` | in-flight workstreams (excludes done/archived) |
| `orch.py list --all` | every workstream |
| `orch.py show <id>` | full JSON + file paths for one workstream |
| `orch.py set <id> phase <phase>` | move phase |
| `orch.py set <id> status <status>` | set status |
| `orch.py set <id> branch <name>` | record the build branch/worktree |
| `orch.py round <id> +1` | bump the round counter |
| `orch.py round <id>` | print the current round |
| `orch.py log <id> "<msg>"` | append a timestamped log entry |
| `orch.py path <id> [dir\|spec\|reviews\|notes]` | print a path (for use in other commands) |
| `orch.py config` | print all config |
| `orch.py config <key>` | print one value (dotted keys index nested dicts, e.g. `models.reviewer`) |
| `orch.py config <key> <value>` | set one value (`true`/`false` parsed as bool; dotted keys set nested, e.g. `models.senior claude-opus-4-8`) |

Use `orch.py path` to get file locations for `codex`, `git`, and subagent
delegation messages rather than hand-building paths.

## Config knobs

| Key | Default | Purpose |
|-----|---------|---------|
| `auto_advance_to_build` | `false` | if false, stop at the approval gate after spec |
| `auto_advance_to_integrate` | `true` | if false, pause for human review before integrating |
| `use_codex` | `true` | gate reviews on the `reviewer` tier (gpt-5.5 / Codex) when present |
| `codex_cmd` | `codex exec --full-auto -s read-only` | base Codex command for `codex exec` calls |
| `test_cmd` | `""` | e.g. `npm test`, `pytest -q`; blank => skip tests (warn) |
| `primary_branch` | `main` | branch you integrate into |
| `memory_file` | `.orchestrate/memory.md` | long-term memory |
| `backlog_file` | `.orchestrate/backlog.md` | backlog / follow-ups |
| `tracker_file` | `.orchestrate/STATUS.md` | status tracker (auto-generated) |
| `models.<tier>` | see below | model per capability tier (`staff`/`senior`/`junior`/`reviewer`/`mechanical`); the `reviewer`/`mechanical` values are the codex model |

Models are keyed by **capability tier**, not named role. Tier defaults:
`staff` `claude-fable-5` (Manager — recommended session model, user's choice — +
Architect), `senior` `opus` (Builder + structure/spec-conformance preflight),
`junior` `claude-sonnet-5` (simple tasks + the codex-runner wrapper), `reviewer`
`gpt-5-codex-max` (gpt-5.5 correctness review via `codex review`, outside the
Claude hierarchy; blank when `use_codex` is off), `mechanical` `gpt-5-codex-max`
(gpt-5.5 computer-use + bulk work). The staff/senior/junior defaults mirror the
agent frontmatter; if you change one, change it in both places
(`.claude/agents/<role>.md` and here) so they don't drift. Full rule:
`@prompts/model-selection.md`.

## Discipline

- Update phase/round/status **before** you end a turn, so a resumed session
  reads the right state.
- Log the outcome of every round (`orch.py log`) — it's the audit trail the
  human and the next session rely on.
- Treat `state.json` as owned by the script; don't hand-edit it.
