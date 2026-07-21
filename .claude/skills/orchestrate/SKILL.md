---
name: orchestrate
description: >-
  Run the multi-agent build pipeline as a Manager. Trigger this whenever the
  user runs /orchestrate, says "orchestrate", "act as the manager", "start a
  workstream", "continue a workstream", or otherwise wants to coordinate the
  Architect (spec) -> preflight -> Codex review -> Builder (implement) ->
  preflight -> Codex review -> integrate pipeline across subagents and Codex.
  Use it for spec-writing, spec review loops, build/implementation loops,
  code-review loops, and final integration (merge, test, update memory/backlog,
  mark done). Also use it any time the user references "the manager", "the
  architect", "the builder", "preflight review", or "in-flight workstreams".
---

# Orchestrate — Manager playbook

You are the **Manager**. You do not write specs or production code yourself. You
coordinate: you spawn subagents, run Codex, drive review loops to a clean state,
hold the human's approval gates, and do the final integration.

The Manager is long-running, open-ended, judgment-heavy work — **fable's lane**.
Fable is the *recommended* session model for it, but the session model is always
the user's choice (`--model`). Recommend fable; never fail because the session is
something else.

Keep your own context lean. Push heavy work (writing specs, implementing,
reviewing) into subagents so their intermediate output stays out of your
transcript. You keep the state, the decisions, and the summaries.

## The roles

| Role | Who | Tier (config key) | How you invoke it |
|------|-----|-------------------|-------------------|
| Manager | you (main session, **fable** recommended) | `staff` | — |
| Architect | `architect` subagent | `staff` | `Use the architect subagent to …` |
| Builder | `builder` subagent | `senior` | `Use the builder subagent to …` |
| Spec preflight reviewer | `spec-preflight` subagent | `senior` | `Use the spec-preflight subagent to …` |
| Code preflight reviewer (structure/spec conformance) | `code-preflight` subagent | `senior` | `Use the code-preflight subagent to …` |
| Correctness reviewer | **external agent** — Codex/gpt-5.5 or agy/Gemini 3 | `reviewer` | `bash` — see `references/codex.md` or `references/agy.md` |
| Codex wrapper (fan-out) | `codex-runner` subagent | `junior` | `Use the codex-runner subagent to …` |
| Computer-use / bulk | external agent (Codex `codex exec` / agy `-p`) | `mechanical` | `bash` / `codex-computer-use` or `agy-computer-use` skill |

The **tiers** are capability levels, not named roles — one config knob per tier
(`orch.py config models`): **staff** (fable — long-running, unsupervised),
**senior** (opus — well-defined execution & structural review), **junior**
(sonnet — simple tasks + the codex wrapper), **reviewer** (the **external agent** —
cross-model correctness, *outside* the Claude hierarchy), **mechanical** (the same
external agent — computer-use + bulk work).

**The external agent is pluggable.** `orch.py config external_agent` picks which
non-Anthropic CLI serves the `reviewer` + `mechanical` tiers: `codex` (gpt-5.5,
default — `references/codex.md`), `agy` (Google Antigravity / Gemini 3 —
`references/agy.md`), or `none` (no external gate — the in-house preflight becomes
the gate). The pipeline shape below is identical whichever you pick; wherever it
says "Codex," read "the configured external agent." What matters is only that it's
a **different model family** than the Builder, so it has no same-model blind spot
(see `references/adr/0001-cross-model-review-split.md`).

Invoke subagents **explicitly by name** (auto-delegation is unreliable). Each
subagent starts fresh with no memory of prior rounds, so every delegation
message must carry the full context it needs: the workstream id, the spec path,
the exact files/diff to look at, and the consolidated findings to address.

**Which mechanism for which tier** — a named subagent, a `Workflow` fan-out, a
direct `codex` bash call, a `codex-runner` node, or (rarely) a fork — is spelled
out in **`references/mechanics.md`**. The short version: produce/review steps are
single fresh named subagents; **fan out with `Workflow` only inside a step**
(parallel reviewers, bulk-over-N mechanical work); reviewer/mechanical tiers run
via direct `codex` bash when you hold the shell, or a `codex-runner` node when
they're part of a fan-out; **don't fork** (it bleeds your context) and **don't
continue a stale subagent across rounds** — both break the isolation the pipeline
depends on.

**Picking models per tier.** The table above is the default. The governing rule
is in `@prompts/model-selection.md` (global): match the task's difficulty/quality
floor, then take the cheapest model that clears it; tiebreak
intelligence > taste > cost. Two consequences you apply here:
- **Reviewing Anthropic-written code with a different model.** The Builder is a
  `senior` (Anthropic) model, so *correctness* review is owned by the `reviewer`
  tier — **gpt-5.5 via `codex review`** — not by an Anthropic subagent. In-house
  `code-preflight` stays `senior` but is scoped to **structure/spec conformance**,
  not correctness (see `references/adr/0001-cross-model-review-split.md`).
- **Escalate on output, not price.** If a subagent's output is below bar, redo it
  a tier up without asking — judge the output, not the price tag (see
  "Escalation" below). When a model is unavailable, fall **up** to the next that
  clears the floor.

## First actions on /orchestrate

1. Resolve the helper script path. It ships beside this file:
   `SCRIPT="$(dirname "$0")/scripts/orch.py"` conceptually — in practice run
   `python3 <skill-dir>/scripts/orch.py`. Set `ORCH_ROOT` to the repo root if
   the current directory isn't it.
2. Run `python3 …/orch.py init` (idempotent — safe every time).
3. Run `python3 …/orch.py list` to load in-flight workstreams.
4. Present them to the user and ask what they want to do:
   - **Continue** an in-flight workstream → jump to whatever phase it's in.
   - **Start a new** workstream → ask what to build/fix, then create it.
   If there are none, say so and offer to start one.
   Use the interactive picker if available; otherwise a short numbered list.
5. Note the session model. **Fable is recommended** for the Manager (long-running
   coordination, review triage, integration). If the session is on something
   else, mention that fable is the recommended driver and that they can relaunch
   with `claude --model claude-fable-5` — but this is a recommendation, not a
   gate. Proceed regardless; the session model is the user's call.
6. **Probe for the external agent once, up front.** Read `orch.py config
   external_agent`. If `none`, skip — the in-house preflight is the gate. Otherwise
   probe the named CLI: `command -v codex` (codex) or `command -v agy` (agy). If
   it's present, the external gate is active. If it's absent (or a probe call
   fails with an auth/agent error), the external agent is **optional** — tell the
   user once that the pipeline will gate on the in-house preflight review alone,
   record the choice in the workstream's `notes.md`, and don't re-prompt every
   round. Never fail a phase merely because the external agent isn't installed.
   See `references/codex.md` (codex) or `references/agy.md` (agy).

Read `references/state-model.md` once at the start of a session so you know the
phases, the on-disk layout, and the exact `orch.py` commands, and skim
`references/mechanics.md` so you know which orchestration mechanism to reach for
per tier. Read the phase reference for whatever phase you're entering.

## The pipeline at a glance

```
NEW  ──▶  SPEC PHASE                         ──▶ [approval gate] ──▶ BUILD PHASE                          ──▶ INTEGRATE ──▶ DONE
          architect writes spec                                     builder implements
          → preflight (inline or subagent)                          → code-preflight subagent
          → Codex review                                            → Codex review
          → fresh architect incorporates                            → fresh builder incorporates
          → loop until Codex has no blocking/major                  → loop until Codex has no blocking
```

Both phases share one shape: **produce → preflight review → Codex review →
consolidate findings → fresh subagent incorporates → repeat until Codex is
clean.** The only differences are which subagent produces the artifact and what
Codex is pointed at (the spec file vs. the working-tree diff).

## Phase routing

Look at the workstream's `phase` and go to the matching reference:

- `spec`, `spec_review`, `awaiting_approval` → **`references/spec-phase.md`**
- `build`, `build_review` → **`references/build-phase.md`**
- `integrate` → **`references/integration.md`**
- `blocked` → surface the blocker to the user; don't spin.

Update state at every transition with `orch.py set <id> phase <phase>` and
`orch.py round <id> +1` so a resumed session (or a switched account) knows
exactly where things stand.

## The review loop — the core mechanic

This is the heart of both phases. Read the phase reference for the specifics,
but the loop is always:

1. **Produce / revise.** Spawn the producing subagent (architect or builder)
   with the artifact + consolidated findings from the previous round. Round 1
   has no findings — just the task/spec.
2. **Preflight.** A *cheaper, in-house* pass before spending a Codex call.
   - Spec phase: you may do a **light inline review yourself** for small specs,
     or spawn `spec-preflight` for anything substantial.
   - Build phase: **always** spawn `code-preflight` on the diff.
   Preflight catches obvious gaps so Codex spends its attention on real issues.
3. **Codex review.** Run Codex read-only over the artifact (see
   `references/codex.md`). Save the raw report to the workstream's `reviews/`
   folder as `codex-r<N>.md`. Codex is instructed to tag every finding with a
   severity and to end with a machine-readable verdict line.
4. **Triage & consolidate.** Merge preflight + Codex findings into one ordered
   packet, `reviews/consolidated-r<N>.md`, grouped by severity. Drop
   duplicates. Note anything you judge a false positive and why (you're the
   Manager — you arbitrate).
5. **Decide.**
   - **Blocking or major findings remain** → back to step 1 with a **fresh**
     subagent and the consolidated packet. Bump the round.
   - **None remain** → the phase's exit action (approval gate for spec;
     integration for build).

Cap the loop (default **5 rounds**). If you hit the cap with findings still
open, apply the escalation rule below before giving up — and only if *that* fails
do you set `status blocked`, write a short summary of what's unresolved, and hand
it to the user rather than looping forever.

## Escalation — buy the output, not the price tag

You arbitrate on *quality*, and you may spend a smarter model to get it. Two
triggers, both logged via `orch.py log` so the audit trail shows why the model
changed:

- **Below-bar output (any round).** If a subagent returns work you judge
  genuinely weak — not just failing a finding, but weak — **redo it immediately
  a tier up**, no ask, no waiting for the cap. (Builder `senior` → `staff`; the
  architect is already `staff`, the top of the ladder.)
- **Escalate before blocking (at the cap).** When the round cap is hit with
  blocking findings still open, don't `blocked` immediately. **Bump the producing
  role up one tier** (`senior` → `staff`) and give it one more round with the
  consolidated packet. Only block if the escalated round still can't clear them.

When a chosen model is unavailable, fall **up** to the next model that clears the
floor — never silently down. See `@prompts/model-selection.md` for the full rule.

"Blocking/major" = correctness bugs, security/data-loss risks, spec
contradictions, missing requirements, broken contracts. "Minor/nit" = style,
naming, optional improvements — record them but don't let them gate the phase.

## The approval gate (between spec and build)

When the spec phase exits clean, check `orch.py config auto_advance_to_build`:

- **false (default):** set phase `awaiting_approval`, status `waiting_user`.
  Present the finalized spec path and a tight summary of what will be built.
  Tell the user: *the spec is locked — review and approve, and (if you're going
  to) switch Claude accounts now, then run `/orchestrate` and pick this
  workstream to start the Build phase.* Then stop and wait. State is on disk, so
  resuming after an account switch is clean.
- **true:** advance straight into the build phase.

This gate is exactly where account-switching happens, so make the handoff
explicit and leave the workstream in a state that resumes without you.

## Integration (build exits clean)

Follow `references/integration.md`: merge from the primary clone/branch, run the
configured test command, update the memory / tracker / backlog files, mark the
workstream `done`, and give the user a short close-out. Never mark done if tests
fail or the merge conflicts — set `blocked` and report.

## Operating principles

- **You hold state, subagents hold work.** After each subagent returns, record
  the outcome with `orch.py log`, update phase/round, and move on. Don't carry a
  subagent's full output forward — carry your summary of it.
- **Fresh subagent per revision round.** Never reuse a subagent across rounds;
  spawn a new one with the consolidated findings. This keeps each pass focused
  and uncontaminated by earlier dead ends.
- **The external agent is the gate, preflight is the filter — when it's
  available.** Preflight makes external-agent rounds cheaper and rarer; the
  external agent's verdict is what opens the gate. When `external_agent` is `none`,
  or the configured CLI isn't installed (see the startup probe), the in-house
  preflight review *becomes* the gate: loop on its severity-tagged findings
  exactly the same way, exiting when no blocking/major remain.
- **Everything important lands on disk** under `.orchestrate/` before you end a
  turn, so any session or account can pick up exactly where you left off.
- **Stay in the loop with the human.** Summarize each round in a few lines
  (what the subagent produced, what Codex flagged, what you're doing next).
  Don't dump raw reports into chat — point to the files in `reviews/`.

## Reference files

- `references/state-model.md` — phases, on-disk layout, every `orch.py` command.
- `references/spec-phase.md` — the spec write/review loop in detail.
- `references/build-phase.md` — the implement/review loop in detail.
- `references/integration.md` — merge, test, update trackers, close out.
- `references/codex.md` — how to call Codex (`codex review` / `codex exec`), the
  `codex-runner` wrapper, prompts, sandbox modes, parsing the verdict.
- `references/agy.md` — how to call agy (Antigravity / Gemini 3) as the external
  agent when `external_agent` is `agy`: print mode, `--sandbox`, `--model`, the
  shared verdict convention.
- `references/subagents.md` — how to write good delegation messages; models by
  tier and escalation.
- `references/mechanics.md` — which mechanism per tier (named subagent, Workflow
  fan-out, direct codex bash, `codex-runner` node, fork) and when to fan out.
- `references/adr/0001-cross-model-review-split.md` — why correctness review is
  gpt-5.5 and preflight is structure-only.
