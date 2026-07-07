# CLAUDE.md

Guidance for Claude Code when working in this repo (the `myagent` warehouse).

## Keep the README in sync

Whenever you add, remove, or rename an installable component under `.claude/`
(a skill, agent, prompt, MCP server, or a new `knownTypes` category in
`installer/scan.go`), update `README.md` in the same change:

- **Skills** → add/remove/adjust its row in the **Skills** table.
- **New `.claude/` category** (or a removed one) → update the **Layout** tree.
- Keep the description in the README consistent with the component's own
  `description` (e.g. a skill's `SKILL.md` frontmatter).

The README's Layout block and Skills table are hand-maintained snapshots, so
they drift the moment a component is added or removed unless updated alongside it.
