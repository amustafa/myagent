# Codex — the external reviewer

Codex is OpenAI's CLI (gpt-5.5). You call it from `bash` as the independent,
cross-model reviewer whose verdict gates each phase — and, more generally, as the
way to reach gpt-5.5 for cross-model review of Anthropic-written code and bulk
mechanical work. It runs locally, reads the repo, and (in review mode) must not
modify anything.

## Two subcommands, two jobs

- **`codex review`** — purpose-built, non-interactive code review. Prefer this for
  the build-phase correctness gate. Point it at the base branch:

  ```bash
  REVIEWER=$(python3 .../orch.py config models.reviewer)   # gpt-5.5 model; may be blank
  codex review --base <primary_branch> ${REVIEWER:+-c model="$REVIEWER"} "<review instructions>"
  codex review --uncommitted ${REVIEWER:+-c model="$REVIEWER"} "<review instructions>"
  ```

  The `reviewer` tier model comes from `orch.py config models.reviewer`; blank
  means use Codex's default.

- **`codex exec`** — general non-interactive agent run, for everything that isn't a
  branch review (spec review, computer-use/bulk **mechanical** work, one-off gpt-5.5
  delegation). Pin its model with `-m $(orch.py config models.mechanical)` for
  mechanical/bulk work, or `models.reviewer` for a spec review. Covered just below.

## Reaching gpt-5.5 from a subagent (the `codex-runner` wrapper)

Sometimes you want gpt-5.5 as a *node in a fan-out* (a Workflow `parallel()`
stage, or one of several Agent-tool subagents) rather than a direct shell call.
You can't spawn a gpt-5.5 agent directly, so use the **`codex-runner`** subagent —
a sonnet model at low effort whose only job is to run your self-contained prompt
through `codex exec`/`codex review` and hand back the result verbatim. When you
already hold a shell (as the Manager usually does), just call codex yourself; the
wrapper is for fan-outs.

## The command

Claude Code's bash is non-interactive, so use `codex exec` (never bare `codex`,
which opens a TUI and hangs). Use `--full-auto` so it doesn't stall on an
approval prompt, and `-s read-only` so a review can't touch files.

Base command comes from config so the user can tune it:

```bash
CODEX=$(python3 .../orch.py config codex_cmd)          # default: codex exec --full-auto -s read-only
CMODEL=$(python3 .../orch.py config models.reviewer)  # gpt-5.5 reviewer model; may be blank
MFLAG=""; [ -n "$CMODEL" ] && MFLAG="-m $CMODEL"
```

Run and capture (stream progress to stderr, final report to stdout → file):

```bash
$CODEX $MFLAG "<review prompt>" | tee "<reviews-dir>/codex-r<N>.md"
CODEX_EXIT=${PIPESTATUS[0]}
```

Notes:
- Model flags go **after** the `exec` subcommand.
- If the base command already includes `exec`, don't add it again.
- `tee` keeps the raw report on disk in the workstream's `reviews/` folder.
- Prefer capturing exit code via `${PIPESTATUS[0]}` since you're piping.

## Review prompts

Ask for severity tags and a machine-readable verdict so triage and gating are
mechanical. Always end the prompt with the verdict-line instruction.

**Spec review:**

```
Review the design spec at <spec path> as a skeptical staff engineer.
Find: correctness gaps, missing or ambiguous requirements, internal
contradictions, unhandled edge cases, security or data-loss risks, and
done-criteria that aren't testable. Do not rewrite the spec.
For each finding output: [SEVERITY: blocking|major|minor] <file/section> — <issue> — <why it matters>.
End with exactly one line:  VERDICT: PASS   (no blocking/major)  or  VERDICT: CHANGES.
```

**Code review** (with `codex review --base <primary_branch>` the diff is already
scoped, so the instructions are just the *what to look for*; the `git diff` phrasing
below is only needed if you fall back to `codex exec`):

```
Review the changes on this branch vs <primary_branch>
(git diff <primary_branch>...HEAD) as a skeptical staff engineer.
Confirm they correctly implement the spec at <spec path>. Find: bugs, logic
errors, regressions, unhandled edge cases, error-handling gaps, security or
data-loss risks, missing tests, and broken contracts. Do not modify code.
For each finding output: [SEVERITY: blocking|major|minor] file:line — <issue> — <why>.
Also, if any blocking findings exist, exit with a non-zero status.
End with exactly one line:  VERDICT: PASS  or  VERDICT: CHANGES.
```

Adding "exit non-zero if blocking findings exist" lets `--full-auto` propagate a
hard signal through `CODEX_EXIT` — a useful cross-check on the verdict line.

## Reading the result

1. Parse the last `VERDICT:` line — `PASS` or `CHANGES`.
2. Cross-check `CODEX_EXIT` (non-zero ⇒ treat as blocking present, even if the
   verdict text says PASS — trust the stricter signal and re-read).
3. Extract findings by their `[SEVERITY: ...]` tags into your consolidated packet.
4. `PASS` with no blocking/major ⇒ the phase can exit. `CHANGES` ⇒ another round.

You are the arbiter: if a Codex finding is clearly wrong or out of scope, mark it
a false positive in the consolidated packet with a one-line reason rather than
sending the subagent to "fix" a non-issue.

## If Codex isn't available

Codex is **optional**. Probe for it once at session start (see SKILL.md "First
actions"), not lazily mid-loop:

```bash
command -v codex && codex exec --full-auto -s read-only "reply with: ok" | tail -1
```

- **Present** → the external Codex gate is active for every review round.
- **Absent, or the probe fails with an auth/agent error** (not a findings-based
  non-zero exit) → tell the user *once* that the pipeline will gate on the
  in-house preflight review alone, record it in the workstream's `notes.md`, and
  proceed without re-prompting each round. The preflight review's severity-tagged
  findings become the gate: loop until no blocking/major remain.

Never fail or block a phase merely because Codex isn't installed. If Codex was
present at startup but a *specific* review call fails on auth mid-run, surface
that to the user rather than silently downgrading — that's a regression, not the
optional path.

## Optional: structured output

For programmatic parsing you can add `--json` to get JSONL events on stdout, then
extract the final agent message. The severity-tag + verdict-line convention above
is usually enough and is easier to skim in the saved report, so prefer it unless
you specifically need machine parsing.
