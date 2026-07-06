package main

import (
	"fmt"
	"strings"
)

func (m model) View() string {
	// A few screens render their own inline errors; only bail out globally for
	// unexpected ones.
	if m.err != nil && m.screen != screenDir && m.screen != screenNameFlavor {
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
	case screenPickTemplate:
		return m.viewPickTemplate()
	case screenFlavorForm:
		return m.form.view()
	case screenNameFlavor:
		return m.viewNameFlavor()
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
	for i, r := range m.rows {
		cursor := "  "
		if i == m.cursor {
			cursor = activeStyle.Render("▸ ")
		}
		if r.kind == rowAddFlavor {
			if lastLabel != "Flavors" {
				b.WriteString(groupStyle.Render("Flavors") + "\n")
				lastLabel = "Flavors"
			}
			label := "＋ Add new flavor"
			if i == m.cursor {
				label = activeStyle.Render(label)
			}
			b.WriteString(fmt.Sprintf("%s%s\n", cursor, label))
			continue
		}

		c := r.comp
		if c.Label != lastLabel {
			b.WriteString(groupStyle.Render(c.Label) + "\n")
			lastLabel = c.Label
		}
		check := "[ ]"
		if m.selected[c.RelPath] {
			check = okStyle.Render("[x]")
		}
		name := c.Name
		if isMCP(c) {
			name = mcpServerName(c) // drop the .json for display
		}
		if i == m.cursor {
			name = activeStyle.Render(name)
		}
		b.WriteString(fmt.Sprintf("%s%s %s%s\n", cursor, check, name, m.rowTags(c)))
	}

	if m.flash != "" {
		b.WriteString("\n" + warnStyle.Render(m.flash) + "\n")
	}
	b.WriteString(helpStyle.Render("↑/↓ move · space toggle/add · a all · u update · d delete · enter apply · esc back") + "\n")
	b.WriteString(dimStyle.Render("checked = install · uncheck an installed item to uninstall"))
	return b.String() + "\n"
}

// rowTags renders the trailing status tags for a component row.
func (m model) rowTags(c Component) string {
	var tags []string
	if c.Flavor != nil {
		tags = append(tags, dimStyle.Render("flavored"))
	}
	if m.installed[c.RelPath] {
		if m.selected[c.RelPath] {
			tags = append(tags, okStyle.Render("● installed"))
		} else {
			tags = append(tags, warnStyle.Render("● installed → will uninstall"))
		}
	}
	if c.Flavor != nil && m.updated[c.RelPath] {
		tags = append(tags, warnStyle.Render("update available"))
	}
	if len(tags) == 0 {
		return ""
	}
	return "  " + strings.Join(tags, "  ")
}

func (m model) viewPickTemplate() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Add new flavor") + "\n")
	b.WriteString(dimStyle.Render("pick a flavorable skill to configure") + "\n\n")
	for i, t := range m.templates {
		cursor := "  "
		name := t.Name
		if i == m.pickCursor {
			cursor = activeStyle.Render("▸ ")
			name = activeStyle.Render(name)
		}
		desc := ""
		if t.Schema != nil {
			desc = dimStyle.Render(fmt.Sprintf("  %d options", len(t.Schema.Options)))
		}
		b.WriteString(fmt.Sprintf("%s%s%s\n", cursor, name, desc))
	}
	b.WriteString(helpStyle.Render("↑/↓ move · enter configure · esc back"))
	return b.String() + "\n"
}

func (m model) viewNameFlavor() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Name this flavor") + "\n\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("a named recipe from %q — must be unique", m.pendingTpl.Name)) + "\n\n")
	b.WriteString(m.nameInput.View() + "\n")
	if m.err != nil {
		b.WriteString(warnStyle.Render(m.err.Error()) + "\n")
	}
	b.WriteString(helpStyle.Render("enter generate · esc back to form · ctrl+c quit"))
	return b.String() + "\n"
}

func (m model) viewConflicts() string {
	idx := m.conflictQueue[m.conflictPos]
	item := m.plan[idx]
	choices := choicesForComponent(item.comp, item.state)

	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("Conflict %d of %d", m.conflictPos+1, len(m.conflictQueue))) + "\n\n")
	b.WriteString(activeStyle.Render(item.comp.RelPath) + "\n")
	b.WriteString(dimStyle.Render(describeState(item)) + "\n\n")

	for i, r := range choices {
		cursor := "  "
		label := resolutionLabel(item.comp, r)
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
	if isMCP(item.comp) {
		switch item.state {
		case destLinkedToUs:
			return "already installed — same definition in " + item.linkTarget
		case destOccupied:
			return "a different definition for this server exists in " + item.linkTarget
		}
		return ""
	}
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

func resolutionLabel(c Component, r resolution) string {
	if isMCP(c) {
		switch r {
		case resSkip:
			return "Skip — leave the MCP config untouched"
		case resRemove:
			return "Remove — delete this server from the MCP config (uninstall)"
		case resOverwrite:
			return "Overwrite — replace with this definition"
		case resInstall:
			return "Install — add this server to the MCP config"
		}
		return ""
	}
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
