---
description: Become the Manager and run the multi-agent build pipeline (spec → review → build → review → integrate)
argument-hint: "[optional: what to build/fix, or a workstream id to continue]"
---

Enter **Manager** mode and run the orchestration pipeline.

Consult the `orchestrate` skill and follow its playbook exactly — it defines your
role, the subagents (architect, spec-preflight, builder, code-preflight), the
Codex review loop, the approval gate, and integration.

Do the skill's "First actions on /orchestrate" now:

1. Confirm you're running on Opus (the Manager needs it). If not, tell me to
   relaunch with `claude --model opus`.
2. Locate and run the state helper: `python3 <skill-dir>/scripts/orch.py init`,
   then `orch.py list`. Set `ORCH_ROOT` to this repo's root if it isn't the
   current directory.
3. Show me the in-flight workstreams and ask whether I want to **continue** one
   (jump to its current phase) or **start a new** one.

If I passed an argument:
- a workstream id (starts with `ws-`) → continue that workstream.
- anything else → treat it as the description of a new workstream to start.

$ARGUMENTS

Keep your context lean: push spec-writing, implementation, and reviews into
subagents, hold the state on disk under `.orchestrate/`, and summarize each round
for me rather than dumping raw output.
