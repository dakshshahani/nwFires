//go:build live

package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEscape}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func press(m model, keys ...tea.KeyMsg) model {
	var out tea.Model = m
	for _, k := range keys {
		var _ tea.Cmd
		out, _ = out.Update(k)
	}
	m2, _ := out.(model)
	return m2
}

func runCmds(m model) (model, error) {
	// drain returned cmds via Update path is handled in press; here handle savedMsg manually
	mm, _ := m.Update(savedMsg{})
	return mm.(model), nil
}

func TestFullFlow(t *testing.T) {
	setupDoc(t)
	defer teardownDoc(t)

	m := newModel()
	msg := fetchCmd()
	if ld, ok := msg.(loadDoneMsg); ok {
		mm, _ := m.Update(ld)
		m = mm.(model)
	}
	if m.mode != modeSearch {
		t.Fatalf("mode=%v want search", m.mode)
	}

	// type live
	m = press(m, key("n"), key("w"), key("F"), key("i"), key("r"), key("e"), key("s"), key("-"), key("T"), key("e"), key("s"), key("t"))
	if got := len(m.results); got != 1 {
		ids := []string{}
		for _, r := range m.results { ids = append(ids, r.ID) }
		fl := []string{}
		for _, r := range m.results { fl = append(fl, r.preferredName()+"|"+r.email()+"|"+r.ID) }
		t.Fatalf("live filter: %d results: %v", got, fl)
	}
	if m.query.Value() != "nwFires-Test" {
		t.Fatalf("query=%q", m.query.Value())
	}

	// enter details
	m = press(m, key("enter"))
	if m.mode != modeDetails || m.current == nil {
		t.Fatalf("mode=%v", m.mode)
	}
	if !strings.Contains(m.detailContent, "nwfires-test@example.com") {
		t.Fatalf("details missing email")
	}

	// start editing (press e)
	m = press(m, key("e"))
	if m.mode != modeEditPick {
		t.Fatalf("mode=%v want editPick, status=%q, npaths=%d", m.mode, m.statusMsg, len(m.editPaths))
	}

	// find preferredName in the list and select it
	idx := -1
	for i, p := range m.editPaths {
		if p == "basicInfo.preferredName" {
			idx = i
		}
	}
	if idx == -1 {
		t.Fatalf("preferredName not in edit paths; paths=%v first20=%v", idx, m.editPaths[:min(20, len(m.editPaths))])
	}
	for m.editIdx > idx {
		m = press(m, key("up"))
	}
	for m.editIdx < idx {
		m = press(m, key("down"))
	}
	m = press(m, key("enter"))
	if m.mode != modeEditInput {
		t.Fatalf("mode=%v want editInput", m.mode)
	}

	// type the new value and save
	m = press(m, key("T"), key("e"), key("s"), key("t"), key("N"), key("a"), key("m"), key("e"))
	var saveCmd tea.Cmd
	var out tea.Model = m
	out, saveCmd = out.Update(key("enter"))
	m, _ = out.(model)
	if m.mode != modeDetails {
		t.Fatalf("mode=%v after save, want details", m.mode)
	}
	if saveCmd == nil {
		t.Fatal("no save cmd returned")
	}
	sm := saveCmd()
	if sv, ok := sm.(savedMsg); ok && sv.err != nil {
		t.Fatalf("save err: %v", sv.err)
	}
		mm, _ := m.Update(sm)
		m = mm.(model)
	if !strings.Contains(m.detailContent, "preferredName") || !strings.Contains(m.detailContent, "TestName") {
		t.Fatalf("details not refreshed: %s", m.detailContent[:min(400, len(m.detailContent))])
	}

	// esc levels all the way back to search, then quit
	m = press(m, key("esc"))
	if m.mode != modeSearch {
		t.Fatalf("esc from details: %v", m.mode)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
