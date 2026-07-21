# agy — the external reviewer (Antigravity / Gemini 3)

`agy` is Google's Antigravity CLI (the successor to Gemini CLI), a single Go
binary that exposes Gemini 3.x — plus a few other non-Anthropic models — in the
terminal. When `external_agent` is `agy`, it plays the exact same role Codex does
otherwise: the independent, **cross-model** reviewer whose verdict gates each
phase, and the backend for computer-use / bulk **mechanical** work. It runs
locally, reads the repo, and (in review mode) must not modify anything.

The review **loop** is identical to Codex's — same severity tags, same
`VERDICT:` line, same triage. Only the CLI invocation differs. Read
`references/codex.md` for the loop shape and the shared review prompts; this file
is just the agy-specific *how to call it*.

## No `review` subcommand — use print mode

Unlike `codex review`, agy has no purpose-built review subcommand. A review is a
single non-interactive **print-mode** run over a diff-scoped prompt:

```bash
AGY=$(python3 .../orch.py config agy_cmd)          # default: agy -p --sandbox
AMODEL=$(python3 .../orch.py config models.reviewer)  # Gemini 3 model; may be blank
MFLAG=""; [ -n "$AMODEL" ] && MFLAG="--model \"$AMODEL\""
```

- `-p` / `--print` runs a single prompt non-interactively and prints the response
  (the equivalent of `codex exec`). **Never** run bare `agy` — it opens a TUI and
  hangs Claude Code's non-interactive bash.
- `--sandbox` runs with terminal restrictions on, so a review can't mutate files.
  Keep it for every review call.
- `--model "<name>"` pins the model. Blank ⇒ agy's own default. Run `agy models`
  to list exact names (e.g. `Gemini 3.1 Pro (High)`). Prefer a strong Gemini 3
  Pro tier for correctness review; agy's Claude models are same-family as the
  Builder and weaken the cross-model blind-spot argument.

Because the diff is not auto-scoped for you (there's no `--base`), the prompt
must tell agy which diff to review — use the `git diff <primary_branch>...HEAD`
phrasing from the `codex exec` fallback in `references/codex.md`.

Run and capture (final report to stdout → file):

```bash
eval "$AGY $MFLAG \"<review prompt>\"" | tee "<reviews-dir>/agy-r<N>.md"
AGY_EXIT=${PIPESTATUS[0]}
```

Save the raw report to the workstream's `reviews/` folder as `agy-r<N>.md` (the
Codex path uses `codex-r<N>.md`; keep the backend in the filename so the audit
trail is unambiguous).

## Review prompts

Use the **same** spec-review and code-review prompts as Codex (see
`references/codex.md` → "Review prompts"): ask for `[SEVERITY: blocking|major|minor]`
tags and end with exactly one `VERDICT: PASS` / `VERDICT: CHANGES` line.

One difference: don't rely on "exit non-zero if blocking findings exist." Codex
propagates that hard signal through `--full-auto`; agy's print mode does not
reliably do the same. **For agy, the `VERDICT:` line is the authoritative
signal** — parse it. Treat a non-zero `AGY_EXIT` as a *run failure* (auth,
network, timeout) to surface, not as "blocking findings present."

## Reading the result

1. Parse the last `VERDICT:` line — `PASS` or `CHANGES`.
2. If `AGY_EXIT` is non-zero, the run itself failed (not a findings signal) —
   surface it and re-run rather than treating it as a verdict.
3. Extract findings by their `[SEVERITY: ...]` tags into your consolidated packet.
4. `PASS` with no blocking/major ⇒ the phase can exit. `CHANGES` ⇒ another round.

You are the arbiter: mark a clearly-wrong or out-of-scope finding a false
positive with a one-line reason rather than sending the subagent to "fix" a
non-issue.

## Mechanical / computer-use work

For work that must *act* on the machine (launch apps, drive simulators, take
screenshots), drop `--sandbox` and auto-approve tool calls with
`--dangerously-skip-permissions`, pinning the `mechanical` model:

```bash
agy -p --dangerously-skip-permissions --model "$(orch.py config models.mechanical)" \
  "<self-contained instruction: launch/drive, save artifacts to ./scratch, report 2-4 lines + paths>"
```

This is exactly what the **`agy-computer-use`** skill wraps. Grant the
non-sandboxed run *only* for computer-use — reviews and analysis stay
`--sandbox`.

## If agy isn't available

Probe once at session start (see SKILL.md "First actions"), not lazily mid-loop:

```bash
command -v agy && agy -p --sandbox "reply with: ok" | tail -1
```

- **Present** → the external agy gate is active for every review round.
- **Absent, or the probe fails with an auth/agent error** (not a findings-based
  signal) → tell the user *once* that the pipeline will gate on the in-house
  preflight review alone, record it in the workstream's `notes.md`, and proceed
  without re-prompting each round. The preflight review's severity-tagged
  findings become the gate: loop until no blocking/major remain.

Never fail or block a phase merely because agy isn't installed. If agy was
present at startup but a *specific* call fails on auth mid-run, surface that to
the user rather than silently downgrading — that's a regression, not the optional
path.
