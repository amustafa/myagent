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
	screenDone
)

// planItem pairs a chosen component with the resolution that will be applied
// to it. Free destinations are pre-resolved to resInstall; conflicts are
// resolved interactively on the conflicts screen.
type planItem struct {
	comp       Component
	state      destState
	linkTarget string
	res        resolution
}

type model struct {
	sourceClaude string
	comps        []Component
	commit       string    // source repo commit at launch
	manifest     *Manifest // install-state record for the chosen target

	screen    screen
	scope     int // 0 = global, 1 = project
	selected  map[int]bool
	installed map[int]bool // computed once the target root is known
	cursor    int

	dir        textinput.Model
	candidates []string

	targetClaude string // resolved destination .claude root

	plan          []planItem
	conflictQueue []int // indexes into plan needing a decision
	conflictPos   int
	choiceCursor  int

	results []string
	err     error
	quit    bool
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

func newModel(sourceClaude string, comps []Component) model {
	ti := textinput.New()
	ti.Placeholder = "~/path/to/project   (Tab to complete)"
	ti.Prompt = "› "
	ti.CharLimit = 4096

	return model{
		sourceClaude: sourceClaude,
		comps:        comps,
		commit:       sourceCommit(filepath.Dir(sourceClaude)),
		screen:       screenScope,
		selected:     make(map[int]bool, len(comps)),
		installed:    make(map[int]bool, len(comps)),
		dir:          ti,
	}
}

// enterSelect computes install status against the now-known target root and
// moves to the selection screen. Already-installed components start checked so
// the form reflects reality and re-running is idempotent.
func (m model) enterSelect() model {
	m.manifest = loadManifest(m.targetClaude)
	for i, c := range m.comps {
		state, _ := classifyComponent(m.targetClaude, c)
		isInstalled := state == destLinkedToUs
		m.installed[i] = isInstalled
		m.selected[i] = isInstalled
	}
	m.screen = screenSelect
	return m
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
		} else {
			m.dir.Focus()
			m.screen = screenDir
			return m, textinput.Blink
		}
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
	m.candidates = nil // typing invalidates the last completion hint
	return m, cmd
}

func (m model) updateSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.comps)-1 {
			m.cursor++
		}
	case " ":
		m.selected[m.cursor] = !m.selected[m.cursor]
	case "a":
		all := !m.allSelected()
		for i := range m.comps {
			m.selected[i] = all
		}
	case "esc":
		m.screen = screenScope
	case "enter":
		return m.buildPlan()
	}
	return m, nil
}

func (m model) updateConflicts(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	idx := m.conflictQueue[m.conflictPos]
	choices := choicesForComponent(m.plan[idx].comp, m.plan[idx].state)
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

// buildPlan classifies every selected component's destination, auto-resolving
// free ones and queueing the rest for interactive decisions.
func (m model) buildPlan() (tea.Model, tea.Cmd) {
	m.plan = nil
	m.conflictQueue = nil
	for i, c := range m.comps {
		state, linkTarget := classifyComponent(m.targetClaude, c)
		checked := m.selected[i]
		item := planItem{comp: c, state: state, linkTarget: linkTarget}
		switch {
		case checked && state == destFree:
			item.res = resInstall
		case checked && state == destLinkedToUs:
			item.res = resSkip // already installed — idempotent no-op
		case checked: // destOccupied or destLinkedElse — genuine conflict
			item.res = resSkip // provisional; user decides on the conflicts screen
			m.conflictQueue = append(m.conflictQueue, len(m.plan))
		case !checked && state == destLinkedToUs:
			item.res = resRemove // unchecked an installed component — uninstall
		default:
			continue // unchecked and not installed by us — nothing to do
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

// runPlan applies every resolution in the plan and records the outcome.
func (m model) runPlan() (tea.Model, tea.Cmd) {
	m.results = nil
	for _, item := range m.plan {
		if item.res == resSkip && item.state == destLinkedToUs {
			m.recordInstall(item.comp) // backfill pre-existing installs into the manifest
			m.results = append(m.results, fmt.Sprintf("✓ %s — already installed", item.comp.RelPath))
			continue
		}
		msg, err := applyComponent(m.targetClaude, item.comp, item.res)
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

// recordInstall notes an install in the manifest, tagged by kind so
// reconciliation knows whether to check for a symlink or an MCP config entry.
func (m model) recordInstall(c Component) {
	kind := "symlink"
	if isMCP(c) {
		kind = "mcp"
	}
	m.manifest.record(c.RelPath, InstanceRecord{
		Kind:        kind,
		Source:      c.Source,
		InstalledAt: nowStamp(),
		Commit:      m.commit,
	})
}

func (m model) allSelected() bool {
	for i := range m.comps {
		if !m.selected[i] {
			return false
		}
	}
	return true
}
