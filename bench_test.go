package main

import (
	"fmt"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func synth(n int) []Applicant {
	out := make([]Applicant, n)
	for i := range out {
		out[i] = Applicant{
			ID: fmt.Sprintf("aabbccddeeffggii%06d", i),
			fields: map[string]any{
				"basicInfo": map[string]any{
					"email":          fmt.Sprintf("applicant%06d@students.someuniversity.edu", i),
					"legalFirstName": "First",
					"legalLastName":  fmt.Sprintf("Last%06d", i),
					"preferredName":  fmt.Sprintf("Pref%06d", i),
				},
				"status": map[string]any{"applicationStatus": "applied"},
			},
		}
	}
	return out
}

type benchResult struct {
	latency time.Duration
	results int
}

func doKey(m model, k tea.KeyMsg) (model, benchResult) {
	start := time.Now()
	out, _ := m.Update(k)
	mm, _ := out.(model)
	return mm, benchResult{time.Since(start), len(mm.results)}
}

func doView(m model) (string, time.Duration) {
	start := time.Now()
	v := m.View()
	return v, time.Since(start)
}

func bench(t *testing.T, n int) {
	t.Helper()
	m := newModel()
	m.mode = modeSearch
	m.apps = synth(n)
	m.width, m.height = 100, 30
	m.results = m.apps
	m.query.SetValue("")

	for _, c := range []rune("testing") {
		km := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{c}}
		m2, r := doKey(m, km)
		_, vLat := doView(m2)
		t.Logf("n=%d key %q: update=%v view=%v results=%d", n, c, r.latency, vLat, r.results)
		m = m2
	}
	// arrow navigation
	start := time.Now()
	for i := 0; i < 10; i++ {
		out, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m, _ = out.(model)
	}
	_, vLat := doView(m)
	t.Logf("n=%d 10x down: %v, view=%v", n, time.Since(start), vLat)
}

func TestBenchTyping(t *testing.T) {
	for _, n := range []int{1000, 10000} {
		bench(t, n)
	}
}
