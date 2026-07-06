# compact-smart

`/compact-smart` is a Claude Code skill for preparing cleaner, safer context compaction during long coding sessions.

It helps you continue a long-running objective across milestones without carrying too much noisy conversation history forward.

Instead of compacting blindly, `/compact-smart` creates:

1. A session-scoped state capsule
2. A focused `/compact` command
3. A post-compaction prompt for the next step

The goal is to preserve the important project state while dropping low-value implementation noise.

## Why use this?

Long Claude Code sessions often accumulate a lot of context:

* exploratory discussion
* failed attempts
* verbose tool logs
* temporary debugging
* obsolete plans
* raw diffs
* implementation details that are already in the repo

Normal compaction may preserve too much of the wrong thing or drop important intent.

`/compact-smart` biases compaction toward what matters for future work:

* objective
* current milestone
* architecture
* package/module structure
* file paths
* APIs and contracts
* decisions and rationale
* constraints and non-goals
* failed approaches
* test results
* next action

It intentionally avoids preserving unnecessary raw code details that can be rediscovered from the repo.

## Installation

Create the skill directory:

```bash
mkdir -p .claude/skills/compact-smart
```

Put the skill definition here:

```text
.claude/skills/compact-smart/SKILL.md
```

After that, Claude Code should expose it as:

```text
/compact-smart
```

## Basic workflow

Use `/compact-smart` when you are at a milestone boundary.

Typical flow:

```text
/compact-smart "Implement the next milestone: migrate auth token storage to the encrypted session abstraction."
```

Claude will output:

1. An inferred or supplied direction
2. A session state capsule
3. A `/compact ...` command
4. A post-compaction prompt

Then you do this:

```text
# 1. Copy and run the generated /compact command
/compact ...

# 2. After compaction finishes, paste the generated post-compaction prompt
...
```

This gives the compaction process a clear target and gives the next session a clean continuation prompt.

## Usage examples

### Use with an explicit next direction

```text
/compact-smart Implement the next milestone: add persistent storage for workspace sessions.
```

Use this when you know exactly what should happen after compaction.

This is the preferred mode.

### Use with a named session

```text
/compact-smart --session auth-refactor Migrate auth token storage to the encrypted session abstraction.
```

Use this when you have multiple Claude Code sessions running at the same time.

The session name helps keep the compacted state scoped to one thread of work.

Good session names:

```text
auth-refactor
billing-worker
settings-ui
search-indexer
agent-memory
release-cli
```

### Use with only a session name

```text
/compact-smart --session billing-worker
```

Use this when Claude should infer the next direction, but you still want the output tied to a specific session.

### Use with no arguments

```text
/compact-smart
```

Use this when you want Claude to infer both:

* the session identity
* the next direction

This is convenient, but less reliable than giving an explicit direction.

## Recommended use pattern

At the end of each milestone, do this:

```text
/compact-smart --session <session-name> <next milestone direction>
```

Then run the generated `/compact` command.

Then paste the generated post-compaction prompt.

Example:

```text
/compact-smart --session settings-ui Next, wire the settings form to the saved preferences API and add validation tests.
```

After compaction, the generated prompt might tell Claude to:

* inspect the settings package first
* read the saved preferences API
* avoid redesigning the form model
* continue from the compacted architecture state
* rediscover implementation details from files instead of memory

## What gets preserved?

The skill asks compaction to preserve:

* session identity
* objective
* current milestone status
* next direction
* repo, branch, or worktree context
* packages and modules involved
* files touched or discussed
* architecture and strategy
* important decisions and rationale
* public APIs and interfaces
* schemas, types, commands, configs, env vars, and migrations
* dependency choices and rationale
* constraints and non-goals
* failed approaches and why they failed
* tests and checks run
* known risks, blockers, or unknowns
* what not to redo
* exact next action

## What gets dropped?

The skill asks compaction to drop:

* exploratory chatter
* duplicate discussion
* obsolete branches
* low-value tool logs
* superseded plans
* full code listings
* line-by-line implementation details
* temporary debug snippets
* raw diffs unless still needed
* repeated stack traces after the key error is captured
* implementation details that can be rediscovered from the repo

## Coding-specific strategy

For coding work, the skill follows this rule:

Preserve the architecture, strategy, interfaces, package layout, constraints, decisions, and test status. Drop raw implementation details that already exist in the repo.

This is important because code is usually not the best thing to preserve in compacted memory.

The repo is the source of truth for exact code.

The compacted context should preserve:

* why the code exists
* what shape it should have
* what decisions were made
* what should not be repeated
* where Claude should look next

## Multiple sessions

`/compact-smart` is designed to work when you have several independent Claude Code sessions running at the same time.

For example:

```text
/compact-smart --session auth-refactor Continue migrating auth session storage.
```

```text
/compact-smart --session billing-worker Continue implementing retry handling for invoice jobs.
```

```text
/compact-smart --session settings-ui Continue wiring the preferences screen.
```

Each generated compaction directive is scoped to that session.

The directive explicitly tells Claude:

* this compaction is for this session only
* do not merge state from other sessions
* do not assume another Claude session has the same context
* keep the post-compaction prompt self-contained

## When to use it

Use `/compact-smart` when:

* you are about to run `/compact`
* you are moving from one milestone to another
* the conversation is getting long
* Claude is starting to lose focus
* the session includes a lot of debugging or exploration
* you want to preserve project direction without preserving noisy details
* you are running multiple sessions in parallel

Good moments:

```text
After finishing architecture planning
After landing a major refactor
After debugging a hard issue
Before starting the next milestone
Before switching packages/modules
Before a context window gets too full
```

## When not to use it

Do not use `/compact-smart` when:

* the current task is tiny
* the next step depends heavily on exact unsaved code
* you have not saved important work to disk
* you are still in the middle of a delicate edit
* you need Claude to keep exact recent implementation details in memory

In those cases, finish the edit, save the relevant files, then compact.

## Best practice

Keep durable project state in the repo.

For example:

```text
docs/handoff.md
docs/project-state.md
docs/architecture.md
TODO.md
```

`/compact-smart` is not a replacement for durable documentation.

Think of it this way:

* Repo files are durable memory.
* Compacted context is working memory.
* `/compact-smart` shapes working memory around the next milestone.

## Suggested milestone ritual

At each milestone boundary:

1. Make sure important code is saved.
2. Update any durable project docs if needed.
3. Run `/compact-smart` with a session name and next direction.
4. Copy and run the generated `/compact` command.
5. Paste the generated post-compaction prompt.
6. Continue the next milestone.

Example:

```text
/compact-smart --session agent-memory Next, implement the memory retrieval interface and add unit tests for ranking behavior.
```

## Prompting tips

Be specific about the next milestone.

Better:

```text
/compact-smart --session auth-refactor Next, migrate token refresh to the encrypted session store and update the auth integration tests.
```

Worse:

```text
/compact-smart keep going
```

Better:

```text
/compact-smart --session search-indexer Next, add incremental indexing support for changed files only. Preserve the current indexing architecture and avoid redesigning the storage layer.
```

Worse:

```text
/compact-smart indexing stuff
```

## Mental model

`/compact-smart` does not do the next task.

It prepares Claude to compact well.

The generated post-compaction prompt is what starts the next task.

The sequence is:

```text
/compact-smart -> /compact -> post-compaction prompt -> continue work
```

## Common mistakes

### Mistake 1: Running `/compact` directly

Direct compaction can work, but it may preserve the wrong details.

Use `/compact-smart` first when the task is complex.

### Mistake 2: Not giving a next direction

Claude can infer the next direction, but explicit is better.

Prefer this:

```text
/compact-smart --session release-cli Next, add support for dry-run release previews.
```

### Mistake 3: Preserving too much code

Do not ask compaction to remember large code blocks unless they are not saved anywhere.

The repo should hold code.

The compacted context should hold intent and strategy.

### Mistake 4: Mixing sessions

Use `--session` when working in parallel.

Do not let auth refactor context leak into billing worker context.

### Mistake 5: Treating compaction as durable memory

Compaction is lossy.

Keep important decisions in durable project docs when they matter long term.

## Example full flow

```text
/compact-smart --session auth-refactor Next, replace the legacy token cache with the encrypted session abstraction and update tests.
```

Claude outputs a generated command like:

```text
/compact Preserve the auth-refactor session state only. Preserve objective, next direction, architecture, package/module layout, file paths, APIs, interfaces, constraints, decisions, dependency details, failed approaches, tests/checks, risks, blockers, and what not to redo. Drop exploratory chatter, duplicate discussion, obsolete branches, low-value tool logs, full code listings, line-by-line implementation details, temporary debug snippets, raw diffs, repeated stack traces, and implementation details that can be rediscovered from the repo. Preserve coding intent and architecture, not unnecessary raw implementation.
```

Run that command.

Then paste the generated post-compaction prompt, which will look something like:

```text
Continue the auth-refactor session. The next direction is to replace the legacy token cache with the encrypted session abstraction and update tests. First inspect the auth package, token cache implementation, encrypted session abstraction, and existing auth tests. Do not redesign the session abstraction or repeat rejected approaches from the compacted state. Treat the repo as the source of truth for exact implementation details. Success means token refresh uses the encrypted session abstraction, legacy cache usage is removed or isolated, and relevant tests pass.
```

## Summary

Use `/compact-smart` to make compaction intentional.

The skill helps Claude preserve:

* where you are
* where you are going
* why decisions were made
* what code areas matter
* what not to redo

And it helps Claude drop:

* noisy history
* raw code that lives in the repo
* stale debugging details
* obsolete plans

The result is a cleaner continuation context for long-running coding work.
