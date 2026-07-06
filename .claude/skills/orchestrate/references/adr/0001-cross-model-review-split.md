# Split code review: Opus checks structure, gpt-5.5 checks correctness

Status: accepted

The Builder is an Anthropic model (Opus), so having an Anthropic model also be the
*correctness* reviewer of that code is a same-model blind spot. We therefore split
the build-phase review into two different jobs: the in-house `code-preflight`
subagent (Opus) checks **spec/structure conformance** — does the diff match the
spec, are there obvious gaps — while **correctness** review (bugs, regressions,
edge cases, contract breaks) is owned by **gpt-5.5 via `codex review`**, a
genuinely independent model.

## Considered options

- **Collapse to one gpt-5.5 review.** Simplest and fully honors "review Anthropic
  code with a different model," but loses the cheap in-house filter and hard-couples
  the pipeline to Codex being installed.
- **Make preflight itself a gpt-5.5 pass.** Honors the rule and keeps two stages,
  but both reviewers are then gpt, so the "two independent looks" value thins out.
- **Split by job (chosen).** Opus owns structure/spec-conformance (a task it's fine
  at even on its own team's code); gpt-5.5 owns correctness. No blind-spot overlap,
  two genuinely different lenses.

## Consequences

Preflight is no longer a correctness gate — if Codex is unavailable, the pipeline
degrades gracefully to the structural check alone (still useful) and the Manager is
told correctness wasn't independently verified, rather than pretending Opus covered
it. When Codex is present, its verdict remains the gate that opens the phase.
