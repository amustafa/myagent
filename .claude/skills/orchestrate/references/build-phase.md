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

### Step A — preflight code review (always a subagent)

Unlike the spec phase, always spawn the preflight reviewer here — code needs a
real read, not a glance.

> **Use the `code-preflight` subagent** to review the changes on branch `<BR>`
> for `<id>` against the spec at `<spec path>`.
> Look at the diff (`git diff <primary_branch>...HEAD`). Check: correctness vs.
> the spec, bugs, edge cases, error handling, security/data-loss, test coverage,
> and anything that would break existing behavior. It is read-only — do not edit.
> Return a severity-tagged findings list (blocking / major / minor) with file:line
> references.

Save its findings to `reviews/preflight-code-r<N>.md`.

### Step B — Codex code review

Run Codex read-only over the uncommitted/branch changes. See
`references/codex.md`. In short:

```bash
$(orch.py config codex_cmd) [ -m <codex_model> ] \
  "Review the changes on this branch vs <primary_branch>
   (git diff <primary_branch>...HEAD) as a staff engineer implementing <id>.
   Verify they correctly implement the spec at <spec path>. Flag bugs,
   regressions, unhandled edge cases, security/data-loss, missing tests, and
   contract breaks. Tag each finding blocking|major|minor with file:line.
   End with a line: VERDICT: PASS or VERDICT: CHANGES." \
  | tee <reviews>/codex-code-r<N>.md
```

Instruct Codex to exit non-zero when blocking findings exist so you can also
gate on `$?` if you want a hard signal (see codex.md).

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

**Round cap:** default 5. If blocking findings persist at the cap, set
`status blocked`, summarize what's unresolved, and bring in the user.

## Exit

When Codex is clean, check `orch.py config auto_advance_to_integrate`:
- **true (default):** proceed to `integration.md`.
- **false:** set `status waiting_user`, present the branch + a change summary,
  and wait for the user to say go.
