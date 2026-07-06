package main

import (
	"fmt"
	"strings"
)

func (m model) View() string {
	if m.err != nil && m.screen != screenDir {
		return fmt.Sprintf("error: %v\n", m.err)
	}
	switch m.screen {
	case screenScope:
		return m.viewScope()
	case screenDir:
		return m.viewDir()
	case screenSelect:
		return m.viewSelect()
	case screenConflicts:
		return m.viewConflicts()
	case screenDone:
		return m.viewDone()
	}
	return ""
}

func (m model) viewScope() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Install myagent components") + "\n\n")
	b.WriteString("Install into which namespace?\n\n")

	opts := []struct{ label, hint string }{
		{"Global", "~/.claude — available in every project"},
		{"Project", "<dir>/.claude — this project only"},
	}
	for i, o := range opts {
		marker := "  "
		label := o.label
		if i == m.scope {
			marker = activeStyle.Render("▸ ")
			label = activeStyle.Render(label)
		}
		b.WriteString(fmt.Sprintf("%s%s  %s\n", marker, label, dimStyle.Render(o.hint)))
	}
	b.WriteString(helpStyle.Render("←/→ or tab toggle · enter confirm · ctrl+c quit"))
	return b.String() + "\n"
}

func (m model) viewDir() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Project directory") + "\n\n")
	b.WriteString("Where is the project? Its .claude/ will hold the symlinks.\n\n")
	b.WriteString(m.dir.View() + "\n")

	if m.err != nil {
		b.WriteString(warnStyle.Render(m.err.Error()) + "\n")
	}
	if len(m.candidates) > 0 {
		b.WriteString("\n" + dimStyle.Render("candidates: "+strings.Join(m.candidates, "  ")) + "\n")
	}
	b.WriteString(helpStyle.Render("tab complete · enter confirm · esc back · ctrl+c quit"))
	return b.String() + "\n"
}

func (m model) viewSelect() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Select components") + "\n")
	b.WriteString(dimStyle.Render("into "+m.targetClaude) + "\n")

	lastLabel := ""
	for i, c := range m.comps {
		if c.Label != lastLabel {
			b.WriteString(groupStyle.Render(c.Label) + "\n")
			lastLabel = c.Label
		}
		cursor := "  "
		if i == m.cursor {
			cursor = activeStyle.Render("▸ ")
		}
		check := "[ ]"
		if m.selected[i] {
			check = okStyle.Render("[x]")
		}
		name := c.Name
		if i == m.cursor {
			name = activeStyle.Render(name)
		}
		tag := ""
		if m.installed[i] {
			tag = "  " + okStyle.Render("● installed")
			if !m.selected[i] {
				tag = "  " + warnStyle.Render("● installed → will uninstall")
			}
		}
		b.WriteString(fmt.Sprintf("%s%s %s%s\n", cursor, check, name, tag))
	}
	b.WriteString(helpStyle.Render("↑/↓ move · space toggle · a all/none · enter apply · esc back") + "\n")
	b.WriteString(dimStyle.Render("checked = install · uncheck an installed item to uninstall"))
	return b.String() + "\n"
}

func (m model) viewConflicts() string {
	idx := m.conflictQueue[m.conflictPos]
	item := m.plan[idx]
	choices := choicesFor(item.state)

	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("Conflict %d of %d", m.conflictPos+1, len(m.conflictQueue))) + "\n\n")
	b.WriteString(activeStyle.Render(item.comp.RelPath) + "\n")
	b.WriteString(dimStyle.Render(describeState(item)) + "\n\n")

	for i, r := range choices {
		cursor := "  "
		label := resolutionLabel(r)
		if i == m.choiceCursor {
			cursor = activeStyle.Render("▸ ")
			label = activeStyle.Render(label)
		}
		b.WriteString(fmt.Sprintf("%s%s\n", cursor, label))
	}
	b.WriteString(helpStyle.Render("↑/↓ choose · enter confirm · ctrl+c quit"))
	return b.String() + "\n"
}

func (m model) viewDone() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Done") + "\n\n")
	for _, r := range m.results {
		if strings.HasPrefix(r, "✗") {
			b.WriteString(warnStyle.Render(r) + "\n")
		} else {
			b.WriteString(r + "\n")
		}
	}
	b.WriteString(helpStyle.Render("enter/q quit"))
	return b.String() + "\n"
}

func describeState(item planItem) string {
	switch item.state {
	case destLinkedToUs:
		return "already installed — symlink points at this repo"
	case destLinkedElse:
		return "a symlink already exists here → " + item.linkTarget
	case destOccupied:
		return "a real file/directory already exists here"
	}
	return ""
}

func resolutionLabel(r resolution) string {
	switch r {
	case resSkip:
		return "Skip — leave it untouched"
	case resRemove:
		return "Remove — delete the symlink (uninstall)"
	case resOverwrite:
		return "Overwrite — delete it, then link"
	case resBackup:
		return "Backup — rename to .bak-N, then link"
	case resInstall:
		return "Install — create the symlink"
	}
	return ""
}
