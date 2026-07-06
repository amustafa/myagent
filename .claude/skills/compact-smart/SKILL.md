mkdir -p .claude/skills/compact-smart
cat > .claude/skills/compact-smart/SKILL.md <<'EOF'
---
description: Prepare a session-scoped, coding-aware compaction directive for continuing long-running work across milestones. Use when the user wants to run /compact while preserving the right state for the next step.
argument-hint: "[optional: --session <name>] [post-compaction direction]"
disable-model-invocation: true
---

# compact-smart

You are preparing this Claude Code session for manual context compaction.

The user may provide:
- a session name using `--session <name>`
- a post-compaction direction in `$ARGUMENTS`
- both
- neither

This skill must support multiple independent sessions running at the same time. Treat each invocation as scoped to one work session.

## Session isolation rules

A work session is a single active thread of work, usually tied to one objective, branch, worktree, milestone, or investigation.

If the user provides `--session <name>`, use that exact name as the session identity.

If no session is provided, infer a concise session identity from the current work, such as:
- branch name
- feature name
- bug name
- package/module name
- milestone name
- objective name

If you cannot infer a meaningful name, use `current-session`.

Do not merge state across unrelated sessions.
Do not assume another Claude session has the same context.
Do not refer to “the other session” unless it was explicitly discussed in this conversation.
Do not create or update shared global state.
Every generated `/compact` directive and post-compaction prompt must be self-contained and tagged with the session identity.

## Goal

Create a high-signal, session-scoped compaction directive that helps `/compact` preserve the information needed to continue this specific work session correctly after compaction.

Do not merely summarize the conversation. Shape the compaction around the next milestone.

## Determine the next direction

If `$ARGUMENTS` contains a post-compaction direction, treat it as the authoritative next direction.

If `$ARGUMENTS` contains only a session name and no direction, infer the most likely next direction from:
- the current objective
- the latest explicit milestone or TODO
- recent decisions
- recent code/file changes
- unresolved blockers
- the user's latest intent

If `$ARGUMENTS` is empty, infer both:
- the session identity
- the next direction

If inferring, say exactly what you inferred and why in one short sentence.

## Coding-aware compaction policy

When the work involves code, compaction should preserve high-level and operational coding context, but should usually drop raw implementation detail.

Preserve:
- architecture and strategy
- package/module layout
- relevant file paths
- public APIs and interfaces
- data models, schemas, types, commands, flags, config keys, env vars, and migrations
- dependency/package choices and why they were chosen
- important invariants
- edge cases and constraints
- tests/checks run and their results
- build/lint/typecheck commands
- known failures and failed approaches
- TODOs and next coding steps
- integration points between packages/modules/services
- user preferences for coding style or project conventions
- exact error messages only when they are still relevant

Usually drop:
- full code listings
- line-by-line implementation details
- obsolete diffs
- temporary debug snippets
- verbose tool logs
- exploratory code that was superseded
- repeated stack traces once the key error is captured
- implementation details that can be rediscovered from the repo
- speculative alternatives that were rejected

Exception: preserve specific code details only when they are not yet written to disk, are hard to reconstruct, or are critical to the next step.

## Produce three sections

### 1. Session state capsule to preserve

Write a compact state capsule with these fields:

- Session identity
- Objective
- Current milestone status
- Next direction
- Repo/branch/worktree context, if known
- Packages/modules involved
- Files touched or discussed
- Architecture and strategy summary
- Important decisions and rationale
- Public APIs, types, schemas, commands, configs, or interfaces established
- Dependencies/package details and rationale
- Constraints and non-goals
- Failed approaches and why they failed
- Tests/checks run and results
- Known risks, blockers, or unknowns
- What not to redo
- What implementation details can be safely rediscovered from the repo
- Exact next action after compaction

Only include facts supported by the current conversation. If something is unknown, mark it as unknown rather than guessing.

### 2. Compaction directive

Create a single copy-pasteable `/compact` command.

The directive must be session-scoped and must tell `/compact` to preserve:
- the session identity
- the state capsule
- the user's objective
- the next direction
- milestone progress
- architecture and strategy
- package/module layout
- file paths
- public APIs and interfaces
- schemas, types, commands, configs, env vars, and migrations
- dependency/package decisions
- tests/checks and results
- failed approaches
- constraints and non-goals
- risks and blockers
- what not to redo
- the post-compaction prompt

The directive must tell `/compact` to drop:
- exploratory chatter
- duplicate discussion
- obsolete branches
- low-value tool logs
- superseded plans
- full code listings
- line-by-line implementation details
- temporary debug snippets
- raw diffs unless still needed
- repeated stack traces after the key error is captured
- implementation details that can be rediscovered from the repo

The directive must explicitly say:
- this compaction is for this session only
- do not merge state from other sessions
- preserve coding intent and architecture, not unnecessary raw implementation

### 3. Post-compaction prompt

Create a self-contained prompt for the user to send after compaction.

The prompt should:
- state the session identity
- restate the next direction
- tell Claude what to inspect first
- tell Claude what not to redo
- define success for the next milestone
- tell Claude to rely on the compacted state plus the repo as source of truth
- tell Claude to rediscover exact implementation details from files instead of relying on lossy memory

## Output format

Use this exact structure:

Inferred or supplied direction:
<one sentence>

Session state capsule:
<state capsule>

Run this next:
```text
/compact <focused compaction directive>
