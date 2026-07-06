package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type screen int

const (
	screenScope screen = iota
	screenDir
	screenSelect
	screenConflicts
	screenPickTemplate
	screenFlavorForm
	screenNameFlavor
	screenDone
)

// planItem pairs a chosen component with the resolution that will be applied
// to it. Free destinations are pre-resolved; conflicts are resolved on the
// conflicts screen.
type planItem struct {
	comp       Component
	state      destState
	linkTarget string
	res        resolution
}

// rowKind distinguishes an installable component row from the action row that
// opens the "add new flavor" flow.
type rowKind int

const (
	rowComponent rowKind = iota
	rowAddFlavor
)

type listRow struct {
	kind rowKind
	comp Component // valid when kind == rowComponent
}

type model struct {
	sourceClaude string
	comps        []Component // basic (non-flavorable) components
	templates    []Template  // flavorable source skills
	commit       string      // source repo commit at launch

	flavors  []FlavorInstance // generated flavors from the global registry
	manifest *Manifest        // install-state record for the chosen target

	screen screen
	scope  int // 0 = global, 1 = project

	rows      []listRow
	selected  map[string]bool // keyed by RelPath
	installed map[string]bool // keyed by RelPath
	updated   map[string]bool // flavor RelPath -> update available
	cursor    int

	dir        textinput.Model
	candidates []string

	targetClaude string

	// flavor-create flow
	form       flavorForm
	pickCursor int
	pendingTpl Template
	nameInput  textinput.Model

	plan          []planItem
	conflictQueue []int
	conflictPos   int
	choiceCursor  int

	flash         string
	pendingDelete string // flavor name awaiting a confirm keypress
	results       []string
	err           error
	quit          bool
}

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	groupStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")).MarginTop(1)
	helpStyle   = lipgloss.NewStyle().Faint(true).MarginTop(1)
	activeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	dimStyle    = lipgloss.NewStyle().Faint(true)
)

func newModel(sourceClaude string, comps []Component, templates []Template) model {
	ti := textinput.New()
	ti.Placeholder = "~/path/to/project   (Tab to complete)"
	ti.Prompt = "› "
	ti.CharLimit = 4096

	name := textinput.New()
	name.Prompt = "› "
	name.CharLimit = 128

	return model{
		sourceClaude: sourceClaude,
		comps:        comps,
		templates:    templates,
		commit:       sourceCommit(filepath.Dir(sourceClaude)),
		screen:       screenScope,
		selected:     map[string]bool{},
		installed:    map[string]bool{},
		updated:      map[string]bool{},
		dir:          ti,
		nameInput:    name,
	}
}

// installables is the flat list of everything that can be symlinked into a
// target: basic components plus each generated flavor rendered dir.
func (m model) installables() []Component {
	out := append([]Component(nil), m.comps...)
	for _, f := range m.flavors {
		out = append(out, f.asComponent())
	}
	return out
}

// enterSelect loads registry + manifest for the chosen target and builds the
// list. Already-installed items start checked so re-running is idempotent.
func (m model) enterSelect() model {
	m.manifest = loadManifest(m.targetClaude)
	m.flavors = listFlavors()
	m.refresh()
	m.screen = screenSelect
	m.cursor = 0
	return m
}

// refresh recomputes install/update status and rebuilds the row list. It
// preserves existing selections and defaults new rows to their installed state.
func (m *model) refresh() {
	for _, c := range m.installables() {
		state, _ := classifyDest(m.targetClaude, c)
		isInstalled := state == destLinkedToUs
		m.installed[c.RelPath] = isInstalled
		if _, ok := m.selected[c.RelPath]; !ok {
			m.selected[c.RelPath] = isInstalled
		}
		if c.Flavor != nil {
			m.updated[c.RelPath] = c.Flavor.updateAvailable(m.commit)
		}
	}
	m.rows = m.rows[:0]
	for _, c := range m.comps {
		m.rows = append(m.rows, listRow{kind: rowComponent, comp: c})
	}
	for _, f := range m.flavors {
		m.rows = append(m.rows, listRow{kind: rowComponent, comp: f.asComponent()})
	}
	m.rows = append(m.rows, listRow{kind: rowAddFlavor})
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.quit = true
			return m, tea.Quit
		}
		switch m.screen {
		case screenScope:
			return m.updateScope(msg)
		case screenDir:
			return m.updateDir(msg)
		case screenSelect:
			return m.updateSelect(msg)
		case screenConflicts:
			return m.updateConflicts(msg)
		case screenPickTemplate:
			return m.updatePickTemplate(msg)
		case screenFlavorForm:
			return m.updateFlavorForm(msg)
		case screenNameFlavor:
			return m.updateNameFlavor(msg)
		case screenDone:
			if msg.Type == tea.KeyEnter || msg.String() == "q" {
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m model) updateScope(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left", "right", "tab", "h", "l":
		m.scope = 1 - m.scope
	case "enter":
		if m.scope == 0 { // global
			home, err := os.UserHomeDir()
			if err != nil {
				m.err = err
				return m, tea.Quit
			}
			m.targetClaude = filepath.Join(home, ".claude")
			return m.enterSelect(), nil
		}
		m.dir.Focus()
		m.screen = screenDir
		return m, textinput.Blink
	}
	return m, nil
}

func (m model) updateDir(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyTab:
		completed, cands := completeDir(m.dir.Value())
		m.dir.SetValue(completed)
		m.dir.CursorEnd()
		m.candidates = cands
		return m, nil
	case tea.KeyEsc:
		m.candidates = nil
		m.screen = screenScope
		return m, nil
	case tea.KeyEnter:
		dir := expandHome(strings.TrimSpace(m.dir.Value()))
		if dir == "" {
			m.err = fmt.Errorf("please enter a project directory")
			return m, nil
		}
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			m.err = fmt.Errorf("not a directory: %s", dir)
			return m, nil
		}
		m.err = nil
		m.candidates = nil
		m.targetClaude = filepath.Join(dir, ".claude")
		return m.enterSelect(), nil
	}
	var cmd tea.Cmd
	m.dir, cmd = m.dir.Update(msg)
	m.candidates = nil
	return m, cmd
}

func (m model) updateSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() != "d" {
		m.pendingDelete = "" // any non-'d' key cancels a pending delete
	}
	row := m.rows[m.cursor]
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		m.flash = ""
	case "down", "j":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}
		m.flash = ""
	case " ":
		if row.kind == rowAddFlavor {
			return m.openCreate()
		}
		m.selected[row.comp.RelPath] = !m.selected[row.comp.RelPath]
	case "a":
		all := !m.allSelected()
		for _, c := range m.installables() {
			m.selected[c.RelPath] = all
		}
	case "u":
		if row.kind == rowComponent && row.comp.Flavor != nil && m.updated[row.comp.RelPath] {
			return m.doUpdate(*row.comp.Flavor)
		}
	case "d":
		if row.kind == rowComponent && row.comp.Flavor != nil {
			return m.doDelete(row.comp.Flavor.Name)
		}
	case "esc":
		m.screen = screenScope
	case "enter":
		if row.kind == rowAddFlavor {
			return m.openCreate()
		}
		return m.buildPlan()
	}
	return m, nil
}

// ---- flavor create flow -----------------------------------------------------

func (m model) openCreate() (tea.Model, tea.Cmd) {
	if len(m.templates) == 0 {
		m.flash = "no flavorable skills found (a skill needs install.py + flavor.json)"
		return m, nil
	}
	m.pickCursor = 0
	m.screen = screenPickTemplate
	return m, nil
}

func (m model) updatePickTemplate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.pickCursor > 0 {
			m.pickCursor--
		}
	case "down", "j":
		if m.pickCursor < len(m.templates)-1 {
			m.pickCursor++
		}
	case "esc":
		m.screen = screenSelect
	case "enter":
		m.pendingTpl = m.templates[m.pickCursor]
		m.form = newFlavorForm(m.pendingTpl.Schema, nil)
		m.screen = screenFlavorForm
	}
	return m, nil
}

func (m model) updateFlavorForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var res formResult
	m.form, cmd, res = m.form.update(msg)
	switch res {
	case frCancel:
		m.screen = screenSelect
	case frSubmit:
		m.nameInput.SetValue(m.pendingTpl.Name)
		m.nameInput.CursorEnd()
		m.nameInput.Focus()
		m.err = nil
		m.screen = screenNameFlavor
		return m, textinput.Blink
	}
	return m, cmd
}

func (m model) updateNameFlavor(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.screen = screenFlavorForm
		return m, nil
	case tea.KeyEnter:
		name := strings.TrimSpace(m.nameInput.Value())
		if name == "" || strings.ContainsAny(name, "/\\ ") {
			m.err = fmt.Errorf("name must be non-empty with no spaces or slashes")
			return m, nil
		}
		inst, err := createFlavor(m.pendingTpl, name, m.form.values, m.commit)
		if err != nil {
			m.err = err
			return m, nil
		}
		m.err = nil
		m.flavors = listFlavors()
		m.refresh()
		m.flash = "created flavor " + inst.Name + " — check it to install here"
		m.screen = screenSelect
		return m, nil
	}
	var cmd tea.Cmd
	m.nameInput, cmd = m.nameInput.Update(msg)
	return m, cmd
}

// doUpdate re-renders a flavor from its saved input against the current source.
func (m model) doUpdate(inst FlavorInstance) (tea.Model, tea.Cmd) {
	updated, err := updateFlavor(inst, m.commit)
	if err != nil {
		m.flash = "update failed: " + err.Error()
		return m, nil
	}
	// If it's installed here, refresh the manifest commit too.
	rel := updated.asComponent().RelPath
	if m.installed[rel] {
		m.recordFlavor(updated.asComponent(), updated)
		_ = m.manifest.save()
	}
	m.flavors = listFlavors()
	m.refresh()
	m.flash = "updated flavor " + inst.Name + " (re-rendered at " + m.commit + ")"
	return m, nil
}

// doDelete removes a generated flavor from the registry (and unlinks it from the
// current target if installed). Requires a confirming second 'd'.
func (m model) doDelete(name string) (tea.Model, tea.Cmd) {
	if m.pendingDelete != name {
		m.pendingDelete = name
		m.flash = "press d again to delete flavor " + name
		return m, nil
	}
	m.pendingDelete = ""
	rel := filepath.Join("skills", name)
	if m.installed[rel] {
		_ = os.Remove(filepath.Join(m.targetClaude, rel))
		m.manifest.forget(rel)
		_ = m.manifest.save()
	}
	if err := deleteFlavor(name); err != nil {
		m.flash = "delete failed: " + err.Error()
		return m, nil
	}
	delete(m.selected, rel)
	delete(m.installed, rel)
	delete(m.updated, rel)
	m.flavors = listFlavors()
	m.refresh()
	m.flash = "deleted flavor " + name
	return m, nil
}

func (m model) updateConflicts(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	idx := m.conflictQueue[m.conflictPos]
	choices := choicesFor(m.plan[idx].state)
	switch msg.String() {
	case "up", "k":
		if m.choiceCursor > 0 {
			m.choiceCursor--
		}
	case "down", "j":
		if m.choiceCursor < len(choices)-1 {
			m.choiceCursor++
		}
	case "enter":
		m.plan[idx].res = choices[m.choiceCursor]
		m.conflictPos++
		m.choiceCursor = 0
		if m.conflictPos >= len(m.conflictQueue) {
			return m.runPlan()
		}
	}
	return m, nil
}

// buildPlan classifies every selected component and queues genuine conflicts.
func (m model) buildPlan() (tea.Model, tea.Cmd) {
	m.plan = nil
	m.conflictQueue = nil
	for _, c := range m.installables() {
		state, linkTarget := classifyDest(m.targetClaude, c)
		checked := m.selected[c.RelPath]
		item := planItem{comp: c, state: state, linkTarget: linkTarget}
		switch {
		case checked && state == destFree:
			item.res = resInstall
		case checked && state == destLinkedToUs:
			item.res = resSkip
		case checked:
			item.res = resSkip
			m.conflictQueue = append(m.conflictQueue, len(m.plan))
		case !checked && state == destLinkedToUs:
			item.res = resRemove
		default:
			continue
		}
		m.plan = append(m.plan, item)
	}
	if len(m.plan) == 0 {
		m.results = []string{"Nothing to do."}
		m.screen = screenDone
		return m, nil
	}
	if len(m.conflictQueue) == 0 {
		return m.runPlan()
	}
	m.conflictPos = 0
	m.choiceCursor = 0
	m.screen = screenConflicts
	return m, nil
}

func (m model) runPlan() (tea.Model, tea.Cmd) {
	m.results = nil
	for _, item := range m.plan {
		if item.res == resSkip && item.state == destLinkedToUs {
			m.recordInstall(item.comp)
			m.results = append(m.results, fmt.Sprintf("✓ %s — already installed", item.comp.RelPath))
			continue
		}
		msg, err := apply(m.targetClaude, item.comp, item.res)
		if err != nil {
			m.results = append(m.results, fmt.Sprintf("✗ %s — %v", item.comp.RelPath, err))
			continue
		}
		switch item.res {
		case resInstall, resOverwrite, resBackup:
			m.recordInstall(item.comp)
		case resRemove:
			m.manifest.forget(item.comp.RelPath)
		}
		m.results = append(m.results, fmt.Sprintf("• %s — %s", item.comp.RelPath, msg))
	}
	if err := m.manifest.save(); err != nil {
		m.results = append(m.results, fmt.Sprintf("✗ could not write install state — %v", err))
	} else {
		m.results = append(m.results, dimStyle.Render("state: "+m.manifest.path))
	}
	m.screen = screenDone
	return m, nil
}

// recordInstall notes an install in the manifest, distinguishing basic symlinks
// from flavored (registry-rendered) instances.
func (m model) recordInstall(c Component) {
	if c.Flavor != nil {
		m.recordFlavor(c, *c.Flavor)
		return
	}
	m.manifest.record(c.RelPath, InstanceRecord{
		Kind: "symlink", Source: c.Source, InstalledAt: nowStamp(), Commit: m.commit,
	})
}

func (m model) recordFlavor(c Component, inst FlavorInstance) {
	m.manifest.record(c.RelPath, InstanceRecord{
		Kind:        "flavored",
		Source:      c.Source,
		InstalledAt: nowStamp(),
		Commit:      inst.Meta.Commit,
		Options:     map[string]any{"flavor": inst.Name},
	})
}

func (m model) allSelected() bool {
	for _, c := range m.installables() {
		if !m.selected[c.RelPath] {
			return false
		}
	}
	return true
}
