# Integration

The Manager does this itself — it's coordination, not implementation. Merge the
finished work, verify it, update the project's memory/tracker/backlog, and close
the workstream. Precondition: the build review loop exited clean.

`orch.py set <id> phase integrate`.

## 1. Merge from the primary clone / branch

Bring the build branch into the primary branch. Prefer a clean, reviewable merge.

```bash
PB=$(python3 .../orch.py config primary_branch)   # e.g. main
BR=... # the workstream branch recorded in state (orch.py show <id>)

git switch "$PB"
git pull --ff-only 2>/dev/null || true
git merge --no-ff "$BR" -m "orchestrate: <id> — <short title>"
```

If the merge conflicts or fails: **stop.** `orch.py set <id> status blocked`,
write the conflict summary to `notes.md`, and hand it to the user. Do not force
or improvise a resolution that wasn't reviewed.

## 2. Run tests

```bash
TEST=$(python3 .../orch.py config test_cmd)
if [ -n "$TEST" ]; then eval "$TEST"; else echo "no test_cmd configured — skipping"; fi
```

- Tests pass → continue.
- Tests fail → **do not mark done.** `orch.py set <id> status blocked`, capture
  the failure output in `notes.md`, and report to the user. A merge that breaks
  tests gets fixed (new build round) or reverted, not shipped.
- No `test_cmd` configured → warn the user that nothing was verified and suggest
  setting one with `orch.py config test_cmd "<cmd>"`.

## 3. Update project records

Append, don't overwrite. Paths come from config (`memory_file`, `backlog_file`).

- **Memory** (`memory_file`): a durable note of what shipped and why — the
  decision, the approach, any non-obvious constraints future work should know.
  Keep it to a few lines; memory is for what you'd want to re-read months later,
  not a changelog dump.
- **Backlog** (`backlog_file`): move any deferred items and leftover minor
  findings here as concrete follow-ups. Remove the workstream from any "todo"
  list if you keep one.
- **Tracker** (`STATUS.md`): regenerated automatically by `orch.py` on the next
  state write — no manual edit needed.

Example:

```bash
{
  echo "## <id> — <title> ($(date -u +%Y-%m-%d))"
  echo "- Shipped: <one-line what/why>"
  echo "- Approach: <one line>"
  echo "- Notes: <constraints / gotchas>"
  echo
} >> "$(python3 .../orch.py config memory_file)"
```

## 4. Clean up and close

```bash
git branch -d "$BR" 2>/dev/null || true      # delete the merged branch
# remove the worktree too, if you made one
```

`orch.py set <id> phase done` and `orch.py set <id> status done`.

## 5. Close-out to the user

A short summary, not a wall of text:
- what shipped (one line),
- how it was verified (tests + Codex clean),
- what moved to the backlog,
- offer to start or continue another workstream.

Then stop — don't auto-start the next thing.
