---
name: codex-computer-use
description: >-
  Delegate all computer-use / runtime operations — launching apps, booting
  simulators, taking screenshots, inspecting a running app — to gpt-5.5 via the
  Codex CLI instead of doing them yourself. Trigger whenever a task needs you to
  run or drive software to observe its behavior: "launch the app", "boot the
  simulator", "take a screenshot", "check what it prints at runtime", "reproduce
  this at runtime", or any UI/runtime verification loop. Keeps screenshot blobs
  and build spew out of your context; gpt-5.5 drives the machine, you decide.
---

# Codex computer-use — delegate runtime operations to gpt-5.5

Computer-use is **bulk mechanical work**: boot a simulator, launch a binary,
click through a flow, grab a screenshot, tail a log. gpt-5.5 is effectively free
for this, and pushing it out of your own turn keeps your context clean of
screenshot blobs, build output, and simulator noise. So: **you don't drive the
machine — Codex does, and reports back.**

You are the decider. Codex is the hands.

## When to use this

Any operation that *acts on or observes a running system*:

- launching or quitting apps / binaries
- booting, listing, or driving simulators/emulators (iOS simulator is the
  canonical case; the pattern is platform-agnostic)
- taking screenshots or screen recordings
- runtime inspection: reading a running app's logs, state, network, or output
- reproducing a bug at runtime, or verifying a fix actually works in the app

If the task is "read the code" or "reason about it," that's you. If it's "run it
and see what happens," that's Codex.

## The contract

**Codex acts and writes artifacts to disk; it returns a short text observation
plus the artifact paths. You read an image only when you actually need to see
it.** This is what preserves the context-hygiene win — don't ask Codex to stream
every frame back for you to look at.

1. Write a **self-contained instruction** for gpt-5.5: what to launch/drive, what
   to observe, and **where to save artifacts** (screenshots → PNG files under a
   scratch dir, logs → a text file). Ask it to return a 2–4 line observation and
   the list of paths — not the raw bytes.
2. Run it through Codex with an **executable sandbox** (computer-use has to *do*
   things, so read-only won't work):

   ```bash
   codex exec -m "<codex_model>" -s workspace-write --full-auto \
     "Boot the iOS simulator, launch <app>, navigate to <screen>, and save a
      screenshot to ./scratch/<name>.png. Then report in 2-4 lines what you see
      on screen and return the screenshot path. If anything errors, capture the
      log to ./scratch/<name>.log and report the path."
   ```

   Or spawn the **`codex-runner`** subagent with the same prompt (tell it the
   task must *act*, so it uses `-s workspace-write --full-auto`) when you want
   this as a node in a larger fan-out.

3. Read Codex's text observation. **Only if you need to see the UI yourself**
   (e.g. confirming a visual bug, judging layout/taste) do you `Read` the saved
   screenshot file. Otherwise the text is enough — keep moving.

## Discipline

- Grant the **executable sandbox only for this** — reviews and analysis still run
  read-only. Computer-use is the deliberate exception because it must launch and
  drive real software.
- Keep artifacts in a scratch dir, not scattered through the repo.
- Don't pull screenshots into context reflexively. The default is "trust the text
  observation"; reading the image is a deliberate choice when vision matters.
- If Codex can't reach the simulator/app or the run fails, surface the captured
  log path and the error — don't guess at what the screen showed.
