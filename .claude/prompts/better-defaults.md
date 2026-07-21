# Better defaults — prefer modern local tooling

In-conversation, use Read/Grep/Glob. The moment you drop to **Bash**, prefer the
modern replacement — cleaner, faster, gitignore-aware, machine-parseable output.

| Instead of | Use |
|------------|-----|
| `grep` | `rg` |
| `find` | `fdfind` (Debian/Ubuntu name for `fd`) |
| `sed`/`awk` on JSON | `jq` |
| YAML by hand | `yq` |
| raw GitHub API / web | `gh` |
| picking from a pipe | `fzf` |

**Traps:** `sg` is `setgid` (util-linux), **not** ast-grep. `duf` may be a shell
alias, not the disk tool. Verify with `command -v` before trusting either.

**Skip for agent use:** `eza`/`bat`/`zoxide`/`procs` optimize for a human reader;
their decorated output parses worse than the plain original.
