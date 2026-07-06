# Spec phase

Goal: produce a spec that Codex signs off on with no blocking or major findings,
then hold the approval gate. The Architect (Fable) writes; you (Manager) review,
run Codex, triage, and loop.

Phases covered: `spec` → `spec_review` → `awaiting_approval`.

## Round 1 — write the spec

1. Make sure the workstream exists and you have the task from the user. If the
   user only gave a rough idea, capture the essentials first (what to build/fix,
   constraints, done-criteria) — a couple of clarifying questions, not an
   interrogation. Write these into `notes.md`.
2. `orch.py set <id> phase spec` and `orch.py round <id> +1` (round 1).
3. Spawn the Architect. Its output goes into the workstream's `spec.md`.

   > **Use the `architect` subagent** to write the spec for `<id>`.
   > Task: `<what the user wants built/fixed, with constraints and done-criteria>`.
   > Repo context: `<key files/dirs, existing patterns, anything it must respect>`.
   > Write the spec to: `<orch.py path <id> spec>`.
   > Follow the spec structure in your instructions. Return a 5-line summary and
   > any open questions — do not paste the whole spec back.

4. When it returns, read `spec.md` yourself (skim), record the summary with
   `orch.py log`, and note any open questions it raised. If it raised questions
   that need the user, ask them now before spending a Codex round.

## The review loop (`spec_review`)

`orch.py set <id> phase spec_review`. Then repeat the produce→review→incorporate
cycle until Codex is clean.

Preflight and the Codex review are independent — if you spawn a `spec-preflight`
subagent, run it and the `codex exec` spec review **concurrently** and consolidate
when both return (see `references/mechanics.md`). For a small spec, the inline
preflight below is cheaper than either.

### Step A — preflight (the cheap filter)

Do this before every Codex call. Two options; pick by size/complexity:

- **Light inline review (you, the Manager).** For small or straightforward
  specs, read `spec.md` and jot obvious issues into `reviews/preflight-r<N>.md`:
  internal contradictions, missing done-criteria, undefined terms, unhandled
  edge cases, scope creep, anything under-specified for a builder.
- **Full preflight subagent.** For substantial specs, spawn it:

  > **Use the `spec-preflight` subagent** to review the spec at
  > `<spec path>` for `<id>`.
  > Check: completeness, internal consistency, testability of done-criteria,
  > missing edge cases, and alignment with `<repo context>`.
  > Return a severity-tagged findings list (blocking / major / minor). Do not
  > rewrite the spec. Save nothing — just return findings.

  Save its returned findings to `reviews/preflight-r<N>.md`.

### Step B — Codex review

Run Codex read-only over the spec and save the raw report. See
`references/codex.md` for the exact command and the review prompt. In short:

```bash
$(orch.py config codex_cmd) [ -m $(orch.py config models.reviewer) ] \
  "Review the design spec at <spec path> as a staff engineer. Flag correctness
   gaps, missing requirements, contradictions, unhandled edge cases, security/
   data-loss risks, and untestable criteria. Tag each finding
   blocking|major|minor. End with a line: VERDICT: PASS or VERDICT: CHANGES." \
  | tee <reviews>/codex-r<N>.md
```

### Step C — triage & consolidate

Merge preflight + Codex findings into `reviews/consolidated-r<N>.md`, grouped by
severity, de-duplicated. As Manager you arbitrate: mark any finding you judge a
false positive or out-of-scope, with a one-line reason. What remains under
**blocking** and **major** is what the next round must fix.

### Step D — decide

- **Blocking/major remain** → round += 1, back to "incorporate" below with a
  **fresh** Architect.
- **Clean** (only minors, or nothing) → exit to the approval gate. Roll leftover
  minors into `notes.md` or the backlog so they aren't lost.

### Incorporate (start of each subsequent round)

`orch.py round <id> +1`, then spawn a **fresh** Architect with the consolidated
packet:

> **Use the `architect` subagent** to revise the spec at `<spec path>` for `<id>`.
> Address these findings (edit the spec in place):
> `<paste consolidated-r<N-1>.md>`
> Keep everything not called out. Return a short changelog of what you changed
> per finding, plus anything you chose not to change and why.

Then loop back to Step A.

**Round cap:** default 5. Before you block, apply the **escalation** rule (see
SKILL.md "Escalation"): the Architect is already fable (top of the ladder), so
escalation here means redoing a below-bar spec immediately rather than looping on
the same weak draft, and — if blocking findings persist at the cap despite a
strong draft — bringing the user in. If blocking findings still persist, set
`status blocked`, summarize the sticking points, and bring the user in. Log any
escalation with `orch.py log`.

## Exit — the approval gate

When Codex returns clean:

1. `orch.py set <id> phase awaiting_approval`.
2. Check `orch.py config auto_advance_to_build`:
   - **false (default):** `orch.py set <id> status waiting_user`. Present the
     finalized spec path and a tight "here's what will be built" summary. Tell
     the user the spec is locked, and that this is the natural point to **switch
     Claude accounts** if they're going to — then re-run `/orchestrate` and pick
     this workstream to start the build. Stop and wait.
   - **true:** proceed directly — see `build-phase.md`.

Leave the workstream in a clean, self-describing state either way: the next
session (possibly a different account) must be able to resume from disk alone.
