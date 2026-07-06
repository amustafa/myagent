# myagent installer

A Charm/Bubble Tea TUI that installs this repo's Claude Code components
(skills, commands, agents, …) into the **global** (`~/.claude/`) or a
**project** (`<dir>/.claude/`) namespace using **symlinks** — so the repo stays
the single source of truth and edits propagate everywhere instantly.

## Run

```bash
cd installer
go run .            # launch the TUI
# or build a binary:
go build -o myagent-install . && ./myagent-install
```

It auto-detects the source repo by walking up from the current directory to the
nearest `.claude/`. Override with `-source /path/to/repo`.

```bash
go run . -list      # print discovered components, no TUI
```

## Flow

1. **Namespace** — toggle Global vs Project (`←/→`/`tab`, `enter`).
2. **Project dir** (project only) — type a path with `Tab` completion.
3. **Components** — checklist grouped by type. Already-installed items start
   **checked** and tagged `● installed`.
   - `space` toggle · `a` all/none · `enter` apply.
   - **Check** an item to install it; **uncheck** an installed item to
     **uninstall** it (removes the symlink).
4. **Conflicts** — if a real file or a foreign symlink already occupies a
   destination, you're prompted per item: **skip / overwrite / backup**.
5. **Done** — per-component results.

## Idempotent

Re-running is safe: components already symlinked to this repo are detected,
shown as installed, and left untouched unless you explicitly uncheck them.
