package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	// A few screens render their own inline errors; only bail out globally for
	// unexpected ones.
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
	case screenFlavorForm:
		return m.form.view()
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
		if r.kind == rowTemplate {
			if lastLabel != "Add New Flavor" {
				b.WriteString(groupStyle.Render("Add New Flavor") + "\n")
				lastLabel = "Add New Flavor"
			}
			name := r.tpl.Name
			hint := fmt.Sprintf("  %d options", len(r.tpl.Schema.Options))
			if r.tpl.Target == "mcp" {
				hint += " · mcp"
			}
			if i == m.cursor {
				name = activeStyle.Render(name)
			}
			b.WriteString(fmt.Sprintf("%s%s%s\n", cursor, name, dimStyle.Render(hint)))
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

	body := b.String()
	// Show a side panel for the hovered row: a template's options preview, or a
	// created flavor's selections. Placed to the right when the terminal is wide
	// enough, stacked below otherwise (so it always shows).
	if panel := m.hoverPanel(); panel != "" {
		if m.width == 0 || m.width >= lipgloss.Width(body)+lipgloss.Width(panel)+2 {
			body = lipgloss.JoinHorizontal(lipgloss.Top, body, panel)
		} else {
			body += "\n" + panel
		}
	}

	var f strings.Builder
	if m.flash != "" {
		f.WriteString(warnStyle.Render(m.flash) + "\n")
	}
	f.WriteString(helpStyle.Render("↑/↓ move · space toggle/create · enter apply · a all · e edit · u update · d delete · esc back") + "\n")
	f.WriteString(dimStyle.Render("checked = install · uncheck to uninstall · e/u/d act on a created flavor"))
	return body + "\n" + f.String() + "\n"
}

// hoverPanel returns the side-panel content for the row under the cursor: a
// created flavor's selections. Empty for everything else (basic components and
// the "Add New Flavor" template rows).
func (m model) hoverPanel() string {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return ""
	}
	if r := m.rows[m.cursor]; r.kind == rowComponent && r.comp.Flavor != nil {
		return m.flavorPanel(r.comp.Flavor)
	}
	return ""
}

// flavorPanel renders a flavor instance's saved selections. When the source
// template is still present its option labels/order are used; otherwise it falls
// back to the raw key/value input.
func (m model) flavorPanel(inst *FlavorInstance) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(inst.Name) + "\n")
	b.WriteString(dimStyle.Render("skill: "+inst.Meta.Skill) + "\n")
	if inst.Meta.Commit != "" {
		b.WriteString(dimStyle.Render("commit: "+inst.Meta.Commit) + "\n")
	}
	b.WriteString("\n" + groupStyle.Render("Selections") + "\n")

	if tpl, ok := m.templateFor(inst.Meta.Skill); ok {
		for _, o := range tpl.Schema.Options {
			if !o.visible(inst.Input) {
				continue
			}
			b.WriteString(activeStyle.Render(o.Label) + "\n")
			b.WriteString("  " + flavorValueString(o, inst.Input[o.Key]) + "\n")
		}
	} else {
		keys := make([]string, 0, len(inst.Input))
		for k := range inst.Input {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(activeStyle.Render(k) + ": " + fmt.Sprint(inst.Input[k]) + "\n")
		}
	}
	return panelStyle.Render(strings.TrimRight(b.String(), "\n"))
}

// flavorValueString formats one selection for the panel, mirroring how the form
// summarizes values. Input read back from JSON arrives as []any, so lists are
// normalized here.
func flavorValueString(o FlavorOption, v any) string {
	switch {
	case o.isMulti():
		xs := toStringList(v)
		if len(xs) == 0 {
			return dimStyle.Render("(none)")
		}
		sep := ", "
		if o.Type == OptEnumList {
			sep = " > "
		}
		return strings.Join(xs, sep)
	case o.Type == OptBool:
		if b, ok := v.(bool); ok && b {
			return "yes"
		}
		return "no"
	default:
		s := fmt.Sprint(v)
		if s == "" {
			return dimStyle.Render("(empty)")
		}
		return s
	}
}

func toStringList(v any) []string {
	switch xs := v.(type) {
	case []string:
		return xs
	case []any:
		out := make([]string, 0, len(xs))
		for _, e := range xs {
			out = append(out, fmt.Sprint(e))
		}
		return out
	}
	return nil
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
