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

**Set `model:` on every delegation — never omit it.** An omitted model inherits
your *session* model, which silently pushes low-floor work (searches, mechanical
edits, read-only preflights) onto opus or fable. Inheritance is exactly why
sonnet "never gets picked": the floor test only fires when you name the model. A
fable session must still hand its Explore/search/bulk-edit subagents down to
sonnet — the session model is the ceiling, not the default for what it spawns.

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

> ## ⛔ NEVER USE HAIKU
>
> A hard rule, not a default: Haiku is off the scoreboard on purpose — never for
> the cheapest task, never as a cost tiebreak, never as a fallback. If the floor
> test lands on Haiku, the floor is wrong; use sonnet/gpt-5.5, or fall **up**.

These are defaults, not limits — you have the right to override them. If a
cheaper model's output doesn't meet the bar, **re-run or redo the work with a
smarter model without asking**. Judge by the **output, not the price tag**.

When your chosen model is **unavailable**, fall **up** to the next model that
still clears the floor (smarter, costs more) — never silently down to one that
might not clear it.

## Reaching gpt-5.5 (via the Codex CLI)

gpt-5.5 runs through OpenAI's `codex` CLI. **Pick by where you are:** holding a
shell → call `codex` directly. Inside a fan-out (Workflow / Agent tool) → wrap it
in the `codex-runner` subagent, since you can't spawn a gpt-5.5 agent directly.

**1. Directly from a shell** — the default; use it whenever you already hold Bash:

```bash
# General delegation / bulk mechanical work (read-only by default):
codex exec -m <codex_model> -s read-only "<self-contained prompt>"

# Purpose-built code review against a base branch:
codex review --base <branch> "<review instructions>"
```

`<codex_model>` is configurable (e.g. `gpt-5-codex-max`) — treat "gpt-5.5" as
whatever your Codex model is set to, not a hard-coded string. `codex exec` for
general work, `codex review` for reviews. The sandbox is read-only by default;
widen it (`-s workspace-write`, `--full-auto`) only when the task must actually
*do* things (see the `codex-computer-use` skill).

**2. As a node in a subagent fan-out** (Workflow `parallel()`/`pipeline()`, or a
parallel Agent-tool spawn) — reach for this *only* when you need gpt-5.5 running
concurrently alongside other agents; a lone call belongs in path 1. Use the
**`codex-runner` subagent** (sonnet at low reasoning effort, which just shells out
to `codex exec` and returns the result verbatim). Invoke it by name, or as an
`agentType` in a Workflow stage.

## Playwright / browser automation → always a sonnet subagent

**Never call the Playwright tools (`mcp__…playwright…`) in your own session.**
Every browser command — navigate, click, snapshot, screenshot, evaluate — runs
in a subagent spawned with `model: sonnet`, which drives the browser and returns
**only the conclusion** (what it observed / whether it succeeded), never the raw
snapshot, DOM dump, or screenshot blob.

Why: driving a browser is low-floor mechanical work (sonnet clears the floor),
and its outputs are huge and near-worthless in your context. Delegating keeps the
blobs out of your window — same reason runtime/computer-use goes to the
`codex-computer-use` skill. Give the subagent a self-contained goal ("log in as
X, confirm the dashboard renders the Y widget, report pass/fail + any console
errors"), not a click-by-click script — let it run the whole flow and hand back
the verdict.
