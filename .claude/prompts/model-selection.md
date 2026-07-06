# Picking the right model for workflows, subagents, and orchestration

This governs the model you hand to **delegated work** — the `model:` on a
subagent, a Workflow `agent()`, an Agent-tool spawn, or a Codex call. It does
**not** govern your own interactive session model: that's the user's pick (set
with `/model`), and you never override it.

## The scoreboard

Higher is always better. `cost` is **affordability**, not price — a high number
means the model is cheap to run.

| model | intelligence | taste | cost (affordability) |
|-------|:------------:|:-----:|:--------------------:|
| fable-5   | 9 | 9 | 2 |
| gpt-5.5   | 8 | 5 | 6 |
| opus-4.8  | 7 | 8 | 4 |
| sonnet-5  | 5 | 7 | 5 |

- **intelligence** — how hard a problem you can hand the model and then leave it
  **unsupervised**. Higher = you can walk away from a gnarlier, longer task.
- **taste** — quality of judgment on ui/ux, code quality, api design, and copy.
- **cost** — affordability; a tiebreaker, never a primary reason.

## How to choose: match the floor, then take the cheapest

Do **not** read the table as "pick the highest score." Read it as a floor test:

1. Estimate the task's **difficulty floor** (how much intelligence it needs to be
   done unsupervised) and its **quality bar** (how much taste the output needs).
2. Among the models that **clear both floors**, pick the **cheapest** (highest
   affordability).
3. Only when two candidates both clear the floor does the tiebreak fire, and the
   order is **intelligence > taste > cost**.

So routine, well-bounded work lands on sonnet; hard or long-running autonomous
work climbs to fable; and cost only decides between models that are *already good
enough*.

## The lanes (defaults, not limits)

- **fable** — big, long-running, autonomous tasks: design, orchestration,
  open-ended work you hand off and leave alone.
- **opus** — well-defined tasks that fable hands off: a locked spec to implement,
  a scoped change, a bounded review.
- **sonnet** — simple, low-floor tasks; also the thin wrapper around gpt-5.5 (see
  below).
- **gpt-5.5** — bulk mechanical work (it's effectively free), and **reviewing
  code written by Anthropic models** (a different model catches what a same-model
  reviewer is blind to).
- **never use haiku.**

These are defaults, not limits — you have the right to override them. If a
cheaper model's output doesn't meet the bar, **re-run or redo the work with a
smarter model without asking**. Judge by the **output, not the price tag**.

When your chosen model is **unavailable**, fall **up** to the next model that
still clears the floor (smarter, costs more) — never silently down to one that
might not clear it.

## Reaching gpt-5.5 (via the Codex CLI)

gpt-5.5 runs through OpenAI's `codex` CLI. Two ways in — use whichever fits:

**Directly from a shell** (when you already hold Bash):

```bash
# General delegation / bulk mechanical work (read-only by default):
codex exec -m <codex_model> -s read-only "<self-contained prompt>"

# Purpose-built code review against a base branch:
codex review --base <branch> "<review instructions>"
```

`<codex_model>` is configurable (e.g. `gpt-5-codex-max`) — treat "gpt-5.5" as
whatever your Codex model is set to, not a hard-coded string. Use `codex exec`
for general work and `codex review` for reviews. Widen the sandbox
(`-s workspace-write`, `--full-auto`) only when the task must actually *do*
things (see the `codex-computer-use` skill).

**As a node in a subagent fan-out** (Workflow `parallel()`/`pipeline()`, or the
Agent tool) — you can't spawn a gpt-5.5 agent directly, so use the **`codex-runner`
subagent**: a sonnet model at **low reasoning effort** that does nothing but run
your self-contained prompt through `codex exec` and return the result. Invoke it
by name, or as an `agentType` in a Workflow stage.
