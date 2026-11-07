//go:build live

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Fresh SQLite file that never existed: boot must do the synchronous cold
// fill (fetch + store) and end up in the search UI with data.
func TestLiveColdBoot(t *testing.T) {
	fresh := filepath.Join(t.TempDir(), "brand-new.sqlite")
	os.Remove(fresh)
	t.Setenv("NWFIRES_CACHE", fresh)

	m := newModel()
	var warm bool
	var loadDone bool
	var n int
	for _, msg := range bootMsgs(t, (m.Init())()) {
		switch v := msg.(type) {
		case cacheLoadedMsg:
			warm, n = true, len(v.apps)
		case loadDoneMsg:
			if v.err != nil {
				t.Fatal(v.err)
			}
			loadDone, n = true, len(v.apps)
		}
	}
	if warm {
		t.Fatal("fresh cache reported a warm load")
	}
	if !loadDone || n == 0 {
		t.Fatalf("cold boot: loadDone=%v n=%d", loadDone, n)
	}
	t.Logf("cold boot OK: fetched+stored %d applicants", n)

	// And the file now exists with rows — next boot is warm.
	c, err := openCache()
	if err != nil {
		t.Fatal(err)
	}
	apps, at, _ := c.load()
	if len(apps) != n || at.IsZero() {
		t.Fatalf("snapshot not stored: %d rows, stamp=%v", len(apps), at)
	}
	c.close()
}

// When the cache cannot open (unwritable location), boot must fall back to
// the live fetch path and still work.
func TestLiveBootFallbackWhenCacheBroken(t *testing.T) {
	t.Setenv("NWFIRES_CACHE", "/dev/null/definitely/not-a-dir/x.sqlite")

	m := newModel()
	var gotData bool
	for _, msg := range bootMsgs(t, (m.Init())()) {
		if v, ok := msg.(loadDoneMsg); ok {
			if v.err != nil {
				t.Fatal(v.err)
			}
			gotData = len(v.apps) > 0
		}
		if _, ok := msg.(cacheLoadedMsg); ok {
			t.Fatal("broken cache should not produce a cache load")
		}
	}
	if !gotData {
		t.Fatal("fallback fetch returned no data")
	}
	t.Log("fallback OK: app still boots and fetches live when cache is out of order")
}
