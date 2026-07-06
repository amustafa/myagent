# Delegating to subagents

Each subagent starts **fresh**: it sees only its own system prompt (the body of
its `.claude/agents/<name>.md` file), your delegation message, and the project's
CLAUDE.md — not your conversation, not earlier rounds, not prior files you read.
So the delegation message must be self-sufficient.

## Invoke explicitly

Auto-delegation by description is unreliable. Name the subagent:

> Use the `architect` subagent to …
> Use the `builder` subagent to …
> Use the `spec-preflight` subagent to …
> Use the `code-preflight` subagent to …

## What every delegation message must carry

1. **Workstream id** — so logs/paths line up.
2. **The artifact path** — the spec file to write/read, or the branch/diff to
   review. Get it with `orch.py path <id> spec` / `git diff …`.
3. **The task or the findings** — round 1: the task and constraints. Later
   rounds: paste the consolidated findings packet verbatim.
4. **Context it can't infer** — repo conventions, files likely involved, lint/
   test/build commands, patterns to follow or avoid.
5. **The return contract** — ask for a *summary*, not the full artifact:
   "return a 5-line summary + open questions", "return a changelog per finding",
   "return severity-tagged findings with file:line". This keeps your context lean.

## Fresh subagent every round

Never continue the same subagent across revision rounds. Spawn a new one and
hand it the consolidated findings. Fresh context per round avoids dragging along
earlier dead ends and keeps each pass sharp.

## Keep control at the Manager

The producing subagents (architect, builder) don't spawn their own reviewers —
you orchestrate every review and every Codex call. This keeps one clear state
machine instead of nested loops you can't see into. (The agent files omit the
`Agent` tool so they can't nest anyway.)

## Model per role

Set in each agent's frontmatter; mirrored in `orch.py config models.*`:
architect → `claude-fable-5`, everyone else → `opus`. If you change one, change
both so they don't drift. If Fable is routed/unavailable in your setup, the
architect will fall back to whatever the model string resolves to — the workflow
still works; note it and carry on.
