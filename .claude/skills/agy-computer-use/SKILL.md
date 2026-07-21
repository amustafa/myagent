---
name: agy-computer-use
description: >-
  Delegate all computer-use / runtime operations — launching apps, booting
  simulators, taking screenshots, inspecting a running app — to Gemini 3 via the
  agy (Google Antigravity) CLI instead of doing them yourself. Trigger whenever a
  task needs you to run or drive software to observe its behavior: "launch the
  app", "boot the simulator", "take a screenshot", "check what it prints at
  runtime", "reproduce this at runtime", or any UI/runtime verification loop. The
  agy-backed sibling of codex-computer-use — keeps screenshot blobs and build
  spew out of your context; agy drives the machine, you decide.
---

# agy computer-use — delegate runtime operations to Gemini 3

Computer-use is **bulk mechanical work**: boot a simulator, launch a binary,
click through a flow, grab a screenshot, tail a log. Pushing it out of your own
turn keeps your context clean of screenshot blobs, build output, and simulator
noise. So: **you don't drive the machine — agy does, and reports back.**

You are the decider. agy is the hands.

This is the `agy` (Google Antigravity / Gemini 3) counterpart of the
`codex-computer-use` skill — same contract, different CLI. Reach for whichever
matches your configured external agent; if you use the orchestrate pipeline, that
is `orch.py config external_agent`.

## When to use this

Any operation that *acts on or observes a running system*:

- launching or quitting apps / binaries
- booting, listing, or driving simulators/emulators (iOS simulator is the
  canonical case; the pattern is platform-agnostic)
- taking screenshots or screen recordings
- runtime inspection: reading a running app's logs, state, network, or output
- reproducing a bug at runtime, or verifying a fix actually works in the app

If the task is "read the code" or "reason about it," that's you. If it's "run it
and see what happens," that's agy.

## The contract

**agy acts and writes artifacts to disk; it returns a short text observation plus
the artifact paths. You read an image only when you actually need to see it.**
This is what preserves the context-hygiene win — don't ask agy to stream every
frame back for you to look at.

1. Write a **self-contained instruction** for agy: what to launch/drive, what to
   observe, and **where to save artifacts** (screenshots → PNG files under a
   scratch dir, logs → a text file). Ask it to return a 2–4 line observation and
   the list of paths — not the raw bytes.
2. Run it through agy in **print mode**, and — because computer-use has to *do*
   things — drop the review-only `--sandbox` and auto-approve tool calls with
   `--dangerously-skip-permissions`:

   ```bash
   agy -p --dangerously-skip-permissions [--model "<agy_model>"] \
     "Boot the iOS simulator, launch <app>, navigate to <screen>, and save a
      screenshot to ./scratch/<name>.png. Then report in 2-4 lines what you see
      on screen and return the screenshot path. If anything errors, capture the
      log to ./scratch/<name>.log and report the path."
   ```

   `-p` (print) keeps it non-interactive — **never** run bare `agy`, which opens a
   TUI and hangs. `--model` is optional; blank uses agy's default (run
   `agy models` to list names). In the orchestrate pipeline, pin it with
   `--model "$(orch.py config models.mechanical)"`.

3. Read agy's text observation. **Only if you need to see the UI yourself** (e.g.
   confirming a visual bug, judging layout/taste) do you `Read` the saved
   screenshot file. Otherwise the text is enough — keep moving.

## Discipline

- **`--sandbox` is for reviews; drop it only here.** Computer-use is the
  deliberate exception because it must launch and drive real software. Analysis
  and review runs stay `--sandbox` (see `references/agy.md` in the orchestrate
  skill).
- Keep artifacts in a scratch dir, not scattered through the repo.
- Don't pull screenshots into context reflexively. The default is "trust the text
  observation"; reading the image is a deliberate choice when vision matters.
- If agy can't reach the simulator/app or the run fails, surface the captured log
  path and the error — don't guess at what the screen showed.
- If `agy` isn't installed (`command -v agy` is empty), say so and fall back to
  `codex-computer-use` or driving the operation yourself — don't silently skip
  the runtime check.
