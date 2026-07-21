# Codex — the external reviewer

> **Backend selector.** The external reviewer + mechanical tier is served by
> whichever CLI `orch.py config external_agent` names: `codex` (this file), `agy`
> (`references/agy.md`, Antigravity / Gemini 3), or `none` (no external gate —
> preflight becomes the gate). The review **loop** is identical across backends;
> only the invocation differs. This file is the `codex` how-to.

Codex is OpenAI's CLI (gpt-5.5). You call it from `bash` as the independent,
cross-model reviewer whose verdict gates each phase — and, more generally, as the
way to reach gpt-5.5 for cross-model review of Anthropic-written code and bulk
mechanical work. It runs locally, reads the repo, and (in review mode) must not
modify anything.

## Two subcommands, two jobs

- **`codex review`** — purpose-built, non-interactive code review. Point it at the
  base branch:

  ```bash
  REVIEWER=$(python3 .../orch.py config models.reviewer)   # gpt-5.5 model; may be blank
  codex review --base <primary_branch> ${REVIEWER:+-c model="$REVIEWER"} < /dev/null
  codex review --uncommitted ${REVIEWER:+-c model="$REVIEWER"} < /dev/null
  ```

  The `reviewer` tier model comes from `orch.py config models.reviewer`; blank
  means use Codex's default.

  > ### ⚠️ `--base`/`--uncommitted` cannot be combined with a custom PROMPT
  > In current builds, `codex review --base <branch> "<instructions>"` **errors**:
  > `the argument '--base <BRANCH>' cannot be used with '[PROMPT]'` (a 0-byte report
  > and exit 2 — looks like a failed review). So `codex review` with a base only
  > runs its *default* review; there is no place for security-focused instructions.
  > **When you need a directed review (the usual case — you want to point the
  > reviewer at specific invariants), use `codex exec` instead** with the git-diff
  > phrasing (see "Code review" prompt below). That is the primary path for a
  > *directed* correctness gate; bare `codex review --base` is fine only when a
  > generic pass suffices. Both still require `< /dev/null`.

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

> ### ⛔ ALWAYS redirect stdin: `< /dev/null`
>
> **Every** `codex exec` / `codex review` call MUST end with `< /dev/null`.
> When stdin is left open (which it is in a backgrounded Bash tool call, and
> often in a foreground one), `codex exec` prints `Reading additional input from
> stdin...` and **blocks forever waiting for input that never comes** — the
> prompt argument is treated as a *prefix* and codex waits to append more from
> stdin. The report stays 0 bytes and the task looks "hung." This is a recurring
> mistake; closing stdin is the fix, and it is harmless when stdin was empty
> anyway. If a codex call ever appears to hang, check `codex-r<N>.stderr.log` for
> that "Reading additional input" line — it means you forgot `< /dev/null`; kill
> it (`pkill -f "codex exec"`) and rerun with the redirect.

Base command comes from config so the user can tune it:

```bash
CODEX=$(python3 .../orch.py config codex_cmd)          # default: codex exec --full-auto -s read-only
CMODEL=$(python3 .../orch.py config models.reviewer)  # gpt-5.5 reviewer model; may be blank
MFLAG=""; [ -n "$CMODEL" ] && MFLAG="-m $CMODEL"
```

Run and capture (progress → stderr log, final report → file; **stdin closed**):

```bash
$CODEX $MFLAG "<review prompt>" > "<reviews-dir>/codex-r<N>.md" 2> "<reviews-dir>/codex-r<N>.stderr.log" < /dev/null
CODEX_EXIT=$?
```

Notes:
- **`< /dev/null` is mandatory** (see the box above) — the single most common
  reason a codex review "hangs."
- Redirect to a file rather than `tee` for background runs — a piped `tee` in a
  detached shell can itself keep the pipe open; a plain `>` with `< /dev/null` is
  the robust shape. (Foreground `| tee` is fine *with* `< /dev/null` if you want
  to watch it, capturing exit via `${PIPESTATUS[0]}`.)
- Model flags go **after** the `exec` subcommand.
- If the base command already includes `exec`, don't add it again.
- Keep the raw report on disk in the workstream's `reviews/` folder.

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

**Code review** (via `codex exec`, the primary path for a directed review — see
the `--base`/custom-prompt warning above — the `git diff` phrasing below scopes
the diff since there's no `--base` flag to do it for you):

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
command -v codex && codex exec --full-auto -s read-only "reply with: ok" < /dev/null | tail -1
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
