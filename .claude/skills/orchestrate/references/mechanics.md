# Companion agents & orchestration mechanics

Knowing *which model* a tier uses (`references/subagents.md`) is half of it. This
file is the other half: **which mechanism you use to actually run each piece**,
and when to reach for a fan-out instead of a single subagent.

## The palette

| Mechanism | What it is | Context | Use it for |
|-----------|------------|---------|-----------|
| **Named subagent** | `Use the <name> subagent to …` (Agent tool, `agentType`) | **fresh, isolated** — sees only its system prompt + your message | The default. Producing (architect/builder) and in-house review (spec/code-preflight). One artifact, one pass. |
| **Workflow** | `Workflow` tool — deterministic `parallel()` / `pipeline()` / loops of `agent()` nodes | each node fresh | **Fan-out over multiple independent items or dimensions** at once. Parallel review lenses, bulk mechanical over N files. |
| **Direct bash** | `codex exec` / `codex review` from your shell | n/a | `reviewer` and `mechanical` tiers when *you* already hold the shell (the common case) — a single codex call needs no subagent. |
| **`codex-runner` subagent** | sonnet-low wrapper around codex | fresh | gpt-5.5 as **one node inside a Workflow/Agent fan-out**, where you can't spawn a gpt agent directly. |
| **Fork** | `subagent_type: "fork"` — clones your live context + model | **inherits Manager context** | **Rare here.** Only when a helper genuinely needs your accumulated reasoning and re-briefing a fresh agent would lose nuance. Default to fresh isolation instead. |

Two mechanisms this pipeline deliberately **avoids**:
- **Fork** as a default — it bleeds Manager context into the worker, defeating the
  isolation that keeps each pass sharp. Reach for it only in the rare triage
  deep-dive where continuity beats cleanliness.
- **Continuing a subagent across rounds** (SendMessage / resume) — producers are
  **fresh every round** with the consolidated packet. Never revive a stale one.

## Which mechanism per tier / role

| tier | role | mechanism |
|------|------|-----------|
| `staff` | Manager | you (the main session) |
| `staff` | Architect | named **`architect`** subagent — fresh each round |
| `senior` | Builder | named **`builder`** subagent — fresh each round |
| `senior` | spec / code preflight | named **`spec-preflight`** / **`code-preflight`** subagent |
| `reviewer` | correctness review | **direct bash** `codex review --base` (default) · or a **`codex-runner`** node when reviewing inside a fan-out |
| `mechanical` | computer-use / bulk | **`codex-computer-use`** skill · **direct bash** `codex exec` · or **`codex-runner`** nodes in a Workflow for bulk-over-N |
| `junior` | simple tasks | a light named subagent, or **`codex-runner`** |

> The `reviewer`/`mechanical` rows above assume the `codex` backend. When
> `external_agent` is `agy`, run it as **direct bash** `agy -p` (there's no
> `agy-runner` subagent — the Manager holds the shell, so call it directly) and
> use the **`agy-computer-use`** skill for the mechanical tier. See
> `references/agy.md`.

## Decision guide

- **One artifact, one pass** → single named subagent. (All the produce steps, and
  each single review.)
- **Several independent items / review dimensions at once** → **Workflow**.
  `pipeline()` by default; `parallel()` only when you truly need all results
  together before the next step (e.g. dedup across every finding). This skill's
  instructions are your opt-in to call `Workflow`.
- **Need gpt-5.5 as one of several parallel nodes** → put a **`codex-runner`** node
  in the Workflow (or a bare `codex exec`/`review` if the stage is pure bash).
- **You already hold a shell and it's a single codex call** → just run `codex`
  directly; don't wrap it in a subagent.
- **Escalating a below-bar result** → re-spawn the **same named subagent with a
  model override one tier up** (Agent `model:` / Workflow `agent(..., {model})`).
  Do *not* fork for this — fork can't change the model, and you want a clean redo.
- **A helper truly needs your live context** → fork. Otherwise, fresh subagent.

## Worked patterns

**Parallel review of one step.** Preflight and correctness review are independent,
so run them concurrently instead of in series:

- Simple (you hold the shell): spawn the `code-preflight` subagent *and* run
  `codex review --base <primary>` in the same turn; consolidate when both return.
- As a Workflow (when you want it deterministic or the diff has several modules):
  ```
  parallel([
    () => agent(preflightPrompt, { agentType: 'code-preflight', phase: 'review' }),
    () => agent(codexReviewPrompt, { agentType: 'codex-runner',  phase: 'review' }),
  ])
  ```
  Then you (Manager) triage the merged findings and drive the loop as usual.

**Bulk mechanical over many items** (e.g. a repetitive transform across N files):
```
pipeline(files, f => agent(mechanicalPrompt(f), { agentType: 'codex-runner' }))
```
gpt-5.5 does the cheap work in parallel nodes; you keep only the summary.

**Producing a spec/impl** stays a single fresh subagent per round — no fan-out,
because there's one artifact and the value is a focused, isolated pass.

## The boundary

Workflow fan-out lives **inside a step** (parallel reviewers, bulk items). It does
**not** replace the state machine: you (Manager) still own phase/round transitions,
the disk state under `.orchestrate/`, the human approval gates, and integration.
Keep the durable, resumable state at the Manager; push only the parallelizable
work into a Workflow.
