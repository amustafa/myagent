package main

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

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

	// Only row is "＋ Add new flavor"; cursor should be on it.
	if m.rows[m.cursor].kind != rowAddFlavor {
		t.Fatalf("expected cursor on add-flavor row, got %+v", m.rows[m.cursor])
	}

	m = send(t, m, key(" ")) // open create
	if m.screen != screenPickTemplate {
		t.Fatalf("want screenPickTemplate, got %d", m.screen)
	}
	m = send(t, m, special(tea.KeyEnter)) // pick the stub template
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
