# Build phase

Goal: implement the finalized spec, then drive the code-review loop until Codex
has no blocking findings. The Builder (Opus) implements; you (Manager) review,
run Codex, triage, and loop. Same shape as the spec phase, pointed at code.

Phases covered: `build` → `build_review`. Precondition: the spec is finalized and
approved (or `auto_advance_to_build` is true).

## Set up an isolated workspace

Do the build on a branch (or a git worktree / "primary clone") so integration is
a clean merge and the main branch stays shippable.

```bash
BR="orch/<id>"
git switch -c "$BR" 2>/dev/null || git switch "$BR"
python3 .../orch.py set <id> branch "$BR"
```

If the project uses worktrees, create one and record its path in `notes.md`.
`orch.py set <id> phase build`.

## Round 1 — implement

Spawn the Builder with the finalized spec and the repo context.

> **Use the `builder` subagent** to implement the spec for `<id>`.
> Spec: `<spec path>` (read it fully; implement it faithfully).
> Work on branch `<BR>`. Repo conventions: `<lint/test/build commands, patterns
> to follow, files likely involved>`.
> Implement completely, keep changes scoped to the spec, and run the build/tests
> locally if you can. Return: a summary of what you changed, the list of touched
> files, how you verified it, and anything you deferred or couldn't do.

When it returns, record the summary (`orch.py log`), skim the diff yourself
(`git diff` or `git diff --stat`), and set `orch.py set <id> phase build_review`.

## The review loop (`build_review`)

Repeat produce→review→incorporate until Codex is clean.

Steps A (preflight) and B (correctness review) are **independent** — the structural
subagent and the codex review don't depend on each other, so run them
**concurrently**, not in series: spawn the `code-preflight` subagent *and* kick off
`codex review` in the same turn (or, for a large multi-module diff, fan them out
with a `Workflow` `parallel()`), then consolidate once both return. See
`references/mechanics.md`.

### Step A — preflight: structure & spec conformance (always a subagent)

Always spawn the preflight reviewer here — but its job is **structure/spec
conformance, not correctness**. The Builder is an Anthropic model, so an Anthropic
model checking its correctness is a same-model blind spot; correctness is Codex's
job (Step B). Preflight instead confirms the diff *matches the spec* and has no
obvious structural gaps — a lens Opus is fine at even on its own team's code. See
`references/adr/0001-cross-model-review-split.md`.

> **Use the `code-preflight` subagent** to check the changes on branch `<BR>` for
> `<id>` against the spec at `<spec path>` for **structure and spec conformance**.
> Look at the diff (`git diff <primary_branch>...HEAD`). Check: does it implement
> every spec requirement, are done-criteria covered, is anything from the spec
> missing or out of scope, are there obvious structural gaps (untested
> done-criteria, TODOs, dead ends). You are **not** the correctness reviewer —
> don't chase subtle bugs; flag conformance gaps. Read-only — do not edit.
> Return a severity-tagged list (blocking / major / minor) with file:line refs.

Save its findings to `reviews/preflight-code-r<N>.md`.

### Step B — external-agent correctness review (the gate)

The **external agent** (`orch.py config external_agent`) is the **correctness
authority** — the cross-model reviewer of the Anthropic-written diff, and the
verdict that opens the phase. For the `codex` backend, use `codex exec` with an
explicit `git diff <primary_branch>...HEAD` instruction — **not**
`codex review --base … "<prompt>"`, which errors when a base and a custom prompt
are combined (see `references/codex.md`). Always `< /dev/null`:

```bash
codex exec -s read-only \
  "Review the changes on this branch vs <primary_branch> (run: git diff <primary_branch>...HEAD).
   Verify they correctly implement the spec at <spec path> for <id>.
   Flag bugs, regressions, unhandled edge cases, error-handling gaps,
   security/data-loss, missing tests, and contract breaks. Tag each finding
   blocking|major|minor with file:line. End with: VERDICT: PASS or VERDICT: CHANGES." \
  > <reviews>/codex-code-r<N>.md 2> <reviews>/codex-code-r<N>.stderr.log < /dev/null
```

Set the Codex model with `-c model="$(orch.py config models.reviewer)"` if you're
pinning one (the `reviewer` tier).
Instruct Codex to exit non-zero when blocking findings exist so you can also gate
on `$?` (see codex.md).

For the `agy` backend, run the same review prompt through `agy -p --sandbox`
(there's no `review` subcommand, so scope it with `git diff <primary>...HEAD` in
the prompt) and save to `agy-code-r<N>.md`; parse the `VERDICT:` line for the gate
— see `references/agy.md`. If `external_agent` is `none` or the CLI isn't
installed, correctness wasn't independently verified — say so to the user rather
than treating the Opus structural preflight as if it covered correctness.

### Step C — triage & consolidate

Merge preflight + Codex findings into `reviews/consolidated-code-r<N>.md`,
grouped by severity, de-duped, with false positives marked and reasoned. Blocking
+ major = the fix list.

### Step D — decide

- **Blocking remain** (and major, at your discretion) → round += 1, incorporate
  with a **fresh** Builder.
- **Clean** → exit to integration (`integration.md`). Push leftover minors to the
  backlog.

### Incorporate (start of each subsequent round)

`orch.py round <id> +1`, then a **fresh** Builder:

> **Use the `builder` subagent** to fix the findings for `<id>` on branch `<BR>`.
> Spec: `<spec path>`. Address these (edit in place, keep the rest):
> `<paste consolidated-code-r<N-1>.md>`
> Re-run build/tests locally if you can. Return a changelog per finding plus
> anything you chose not to change and why.

Loop back to Step A.

**Round cap:** default 5. Before you block, **escalate** (see SKILL.md
"Escalation"): if blocking findings persist at the cap, bump the Builder up a
tier (opus → fable) and give it one more round with the consolidated packet;
only set `status blocked` if that escalated round still can't clear them. Also,
any round where the Builder's output is genuinely below bar, redo it with a
smarter model immediately — don't wait for the cap. Log escalations with
`orch.py log`.

## Exit

When Codex is clean, check `orch.py config auto_advance_to_integrate`:
- **true (default):** proceed to `integration.md`.
- **false:** set `status waiting_user`, present the branch + a change summary,
  and wait for the user to say go.
