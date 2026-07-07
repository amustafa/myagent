# Git worktree placement

All git worktrees for a repo MUST be created **inside the repository root** —
never in a sibling or external directory. Two locations apply:

- **General / human-created worktrees** go under `.worktree/` at the repo root
  (e.g. `git worktree add .worktree/my-feature <branch>`). Keep this path out of
  version control via the machine-local `.git/info/exclude` (not `.gitignore`),
  so the convention stays local and is not imposed on collaborators. If the
  entry is missing, add `/.worktree/` to `.git/info/exclude` before creating the
  worktree.

- **Worktrees created by Claude** go under `.claude/worktrees/`
  (e.g. `git worktree add .claude/worktrees/<task> <branch>`). Add
  `.claude/worktrees/` to the committed `.gitignore` if it isn't already there.

Rationale: keeping worktrees inside the repo root under a predictable, ignored
path keeps the checkout self-contained and avoids stray, tracked worktree files.
