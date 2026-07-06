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

## Models by tier

The governing rule is `@prompts/model-selection.md` (global): match the task's
floor, take the cheapest model that clears it, tiebreak intelligence > taste >
cost. Models are configured by **capability tier**, not by named role — one knob
per tier (`orch.py config models`). Each role maps to a tier:

| tier (config key) | default model | roles / use |
|-------------------|---------------|-------------|
| `staff`      | `claude-fable-5`   | Manager (recommended session model), Architect — long-running, unsupervised, taste-heavy |
| `senior`     | `opus`             | Builder, spec-preflight, code-preflight — well-defined execution & structure/spec-conformance review |
| `junior`     | `claude-sonnet-5`  | simple/low-floor tasks; the `codex-runner` wrapper (run at low effort) |
| `reviewer`   | gpt-5.5 via `codex review` | cross-model correctness review of Anthropic code — **outside** the Claude hierarchy |
| `mechanical` | gpt-5.5 via `codex exec`   | computer-use + bulk mechanical work |

Each subagent's frontmatter pins a concrete model that matches its tier; the
`orch.py config models.<tier>` values mirror them. If you change one, change both
so they don't drift.

**Escalation.** These are defaults, not limits. If a producing subagent's output
is below bar, redo it a tier up without asking (Builder `senior` → `staff`; the
architect is already `staff`). At the round cap with blocking findings open, bump
the producer up a tier for one more round before blocking. When a model is
unavailable, fall **up** to the next that clears the floor — never silently down.
Log escalations with `orch.py log`.
