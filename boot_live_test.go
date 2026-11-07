//go:build live

package main

import (
	"os"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// bootMsgs flattens a BatchMsg, running every non-spinner command, and
// collects the resulting messages.
func bootMsgs(t *testing.T, msg tea.Msg) []tea.Msg {
	t.Helper()
	var out []tea.Msg
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			got := c()
			if _, isTick := got.(spinner.TickMsg); isTick {
				continue
			}
			if got != nil {
				out = append(out, got)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	out = append(out, msg)
	return out
}

// Exercises the real boot path: cold run fetches+stores, warm run loads the
// SQLite snapshot instantly.
func TestLiveBootCache(t *testing.T) {
	m := newModel()
	var cached loadable
	for _, msg := range bootMsgs(t, (m.Init())()) {
		switch v := msg.(type) {
		case cacheLoadedMsg:
			cached.applicants, cached.warm = v.apps, true
		case loadDoneMsg:
			if v.err != nil {
				t.Fatal(v.err)
			}
			cached.applicants, cached.warm = v.apps, false
		}
	}
	if cached.warm {
		t.Logf("warm boot: %d applicants from snapshot", len(cached.applicants))
	} else {
		t.Logf("cold boot: fetched %d applicants", len(cached.applicants))
	}
	if len(cached.applicants) == 0 {
		t.Fatal("no applicants after first boot")
	}

	m2 := newModel()
	var warm bool
	var n int
	for _, msg := range bootMsgs(t, (m2.Init())()) {
		if v, ok := msg.(cacheLoadedMsg); ok {
			warm, n = true, len(v.apps)
		}
	}
	if !warm || n == 0 {
		t.Fatalf("second boot: warm=%v n=%d", warm, n)
	}
	t.Logf("warm boot: %d applicants from snapshot", n)
}

type loadable struct {
	applicants []Applicant
	warm       bool
}

func teardownCache(t *testing.T) {
	if p := os.Getenv("NWFIRES_CACHE"); p != "" {
		_ = os.Remove(p)
	}
}
