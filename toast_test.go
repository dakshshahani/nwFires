package main

import (
	"strings"
	"testing"
	"time"
)

func TestRefreshToast(t *testing.T) {
	m := newModel()
	m.mode = modeSearch
	m.width, m.height = 100, 30
	m.apps = []Applicant{seedApplicant("uid1", "a@b.c", "applied")}

	// A successful bg refresh sets the toast and schedules the hide tick.
	out, cmd := m.Update(refreshDoneMsg{apps: m.apps})
	m, _ = out.(model)
	if cmd == nil {
		t.Fatal("no hide-tick cmd scheduled")
	}
	if m.notifExpire.IsZero() || m.notifExpire.Before(time.Now()) {
		t.Fatalf("toast not scheduled: %v", m.notifExpire)
	}
	f := m.footerLine()
	if !strings.Contains(f, "latest snapshot fetched") {
		t.Fatalf("footer lacks toast: %q", f)
	}
	if !strings.HasSuffix(f, "fetched ") {
		t.Fatalf("toast not right-aligned: %q", f)
	}

	// Simulated hide: after expiry the toast disappears and hints remain.
	out, _ = m.Update(notifHideMsg{})
	m, _ = out.(model)
	m.notifExpire = time.Now().Add(-1 * time.Second)
	if m.toast() != "" {
		t.Fatalf("toast persisted after expiry")
	}
	if !strings.Contains(m.footerLine(), "navigate") {
		t.Fatalf("hints lost: %q", m.footerLine())
	}

	// A failed refresh never sets a toast.
	m.notifExpire = time.Time{}
	out, _ = m.Update(refreshDoneMsg{err: errMock{}})
	m, _ = out.(model)
	if !m.notifExpire.IsZero() {
		t.Fatal("failed refresh set a toast")
	}
}

type errMock struct{}

func (errMock) Error() string { return "mock" }
