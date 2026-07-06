package main

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSelectViewListsTemplatesUnderAddNewFlavor(t *testing.T) {
	t.Setenv("MYAGENTCFG_DIR", t.TempDir())
	tpl := stubTemplate(t)
	m := newModel("/src/.claude", nil, []Template{tpl})
	m.targetClaude = filepath.Join(t.TempDir(), ".claude")
	m = m.enterSelect()

	out := m.viewSelect()
	if !strings.Contains(out, "Add New Flavor") {
		t.Errorf("expected an 'Add New Flavor' header, got:\n%s", out)
	}
	if !strings.Contains(out, "stub") {
		t.Errorf("expected the template name listed, got:\n%s", out)
	}
	if strings.Contains(out, "Add new flavor") { // the old single action row
		t.Errorf("old '＋ Add new flavor' action row should be gone:\n%s", out)
	}
}

// panelSetup builds a fresh model with one rendered flavor ("p1", label=hello)
// at the given width, and returns it plus the flavor-row and template-row
// indices. Each caller does a single render (calling viewSelect repeatedly on
// fresh models within one test proved flaky in a way the single-render TUI never
// hits — see the separate tests below).
func panelSetup(t *testing.T, width int) (model, int, int) {
	t.Helper()
	t.Setenv("MYAGENTCFG_DIR", t.TempDir())
	tpl := stubTemplate(t) // one option "label" (registry_test.go)
	if _, err := createFlavor(tpl, "p1", map[string]any{"label": "hello"}, "c1"); err != nil {
		t.Fatal(err)
	}
	m := newModel("/src/.claude", nil, []Template{tpl})
	m.width = width
	m.targetClaude = filepath.Join(t.TempDir(), ".claude")
	m = m.enterSelect()
	flavorRow, templateRow := -1, -1
	for i, r := range m.rows {
		if r.kind == rowComponent && r.comp.Flavor != nil {
			flavorRow = i
		}
		if r.kind == rowTemplate {
			templateRow = i
		}
	}
	if flavorRow < 0 || templateRow < 0 {
		t.Fatalf("expected a flavor row and a template row: %+v", m.rows)
	}
	return m, flavorRow, templateRow
}

func TestFlavorPanelShowsOnFlavorRow(t *testing.T) {
	m, flavorRow, _ := panelSetup(t, 120)
	m.cursor = flavorRow
	out := m.viewSelect()
	if !strings.Contains(out, "Selections") || !strings.Contains(out, "hello") {
		t.Errorf("expected the selections panel with the chosen value, got:\n%s", out)
	}
}

func TestFlavorPanelHiddenOnTemplateRow(t *testing.T) {
	m, _, templateRow := panelSetup(t, 120)
	m.cursor = templateRow
	if strings.Contains(m.viewSelect(), "Selections") {
		t.Error("panel should not appear when hovering a template row")
	}
}

func TestFlavorPanelStacksBelowWhenNarrow(t *testing.T) {
	m, flavorRow, _ := panelSetup(t, 40)
	m.cursor = flavorRow
	// On a narrow terminal the panel stacks below rather than beside — but it
	// still shows, so the info is never hidden.
	if !strings.Contains(m.viewSelect(), "Selections") {
		t.Error("panel should still appear (stacked below) on a narrow terminal")
	}
}

func TestTemplatePanelPreviewsOptions(t *testing.T) {
	m, _, templateRow := panelSetup(t, 120)
	m.cursor = templateRow
	out := m.viewSelect()
	// Hovering a template previews its options (not a created flavor's selections).
	if !strings.Contains(out, "Options") || !strings.Contains(out, "configure & create") {
		t.Errorf("expected a template options preview, got:\n%s", out)
	}
	if strings.Contains(out, "Selections") {
		t.Errorf("template preview should not show instance 'Selections':\n%s", out)
	}
}

func send(t *testing.T, m model, msg tea.Msg) model {
	t.Helper()
	next, _ := m.Update(msg)
	return next.(model)
}

func key(s string) tea.KeyMsg          { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }
func special(k tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: k} }

// Drives the full create-flavor flow through the real model to catch panics and
// verify a flavor is generated and appears on the list.
func TestCreateFlavorFlow(t *testing.T) {
	t.Setenv("MYAGENTCFG_DIR", t.TempDir())
	tpl := stubTemplate(t)

	m := newModel("/src/.claude", nil, []Template{tpl})
	m.targetClaude = filepath.Join(t.TempDir(), ".claude")
	m = m.enterSelect()

	// Only row is the stub template under "Add New Flavor"; cursor is on it.
	if m.rows[m.cursor].kind != rowTemplate {
		t.Fatalf("expected cursor on a template row, got %+v", m.rows[m.cursor])
	}

	m = send(t, m, key(" ")) // selecting a template goes straight to its form
	if m.screen != screenFlavorForm {
		t.Fatalf("want screenFlavorForm, got %d", m.screen)
	}

	// The stub has one option (label, default "hi"); move to Submit and submit.
	m = send(t, m, special(tea.KeyDown))  // onto the Submit row
	m = send(t, m, special(tea.KeyEnter)) // submit -> name screen
	if m.screen != screenNameFlavor {
		t.Fatalf("want screenNameFlavor, got %d (err=%v)", m.screen, m.err)
	}

	m = send(t, m, special(tea.KeyEnter)) // accept default name "stub"
	if m.screen != screenSelect {
		t.Fatalf("want screenSelect after create, got %d (err=%v)", m.screen, m.err)
	}
	if len(m.flavors) != 1 || m.flavors[0].Name != "stub" {
		t.Fatalf("flavor not created: %+v", m.flavors)
	}

	// The new flavor is now an installable row (not yet installed/checked).
	rel := filepath.Join("skills", "stub")
	if m.installed[rel] {
		t.Error("new flavor should not be installed yet")
	}
	found := false
	for _, r := range m.rows {
		if r.kind == rowComponent && r.comp.RelPath == rel {
			found = true
		}
	}
	if !found {
		t.Error("new flavor not present as a component row")
	}
}
