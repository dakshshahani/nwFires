package main

import (
	"os"
	"testing"
)

func testCache(t *testing.T) (*cache, func()) {
	t.Helper()
	dir := t.TempDir()
	f := dir + "/t.sqlite"
	t.Setenv("NWFIRES_CACHE", f)
	c, err := openCache()
	if err != nil {
		t.Fatalf("openCache: %v", err)
	}
	return c, func() { c.close(); _ = os.Remove(f) }
}

func seedApplicant(id, email, status string) Applicant {
	return Applicant{ID: id, Doc: map[string]any{
		"_id": map[string]any{"stringValue": id},
		"basicInfo": map[string]any{"mapValue": map[string]any{"fields": map[string]any{
			"email":          map[string]any{"stringValue": email},
			"legalFirstName": map[string]any{"stringValue": "Test"},
			"legalLastName":  map[string]any{"stringValue": " Applicant"},
		}}},
		"status": map[string]any{"mapValue": map[string]any{"fields": map[string]any{"applicationStatus": map[string]any{"stringValue": status}}}},
	}}
}

func TestCacheRoundTrip(t *testing.T) {
	c, done := testCache(t)
	defer done()

	docs := []Applicant{seedApplicant("uid_test_1", "a@b.c", "applied")}
	if err := c.store(docs); err != nil {
		t.Fatal(err)
	}
	apps, at, lerr := c.load()
	if lerr != nil {
		t.Fatal(lerr)
	}
	if len(apps) != 1 || apps[0].ID != "uid_test_1" {
		t.Fatalf("load: %d apps", len(apps))
	}
	if at.IsZero() {
		t.Fatal("no stamp")
	}
	got, _ := apps[0].valueTree()["basicInfo"].(map[string]any)["email"].(string)
	if got != "a@b.c" {
		t.Fatalf("email round trip: %q", got)
	}
	if !apps[0].matches("a@b.c") || apps[0].matches("zzz") {
		t.Fatal("matching on cached data failed")
	}

	if err := c.saveRow(&apps[0]); err != nil {
		t.Fatal(err)
	}
	apps2, _, _ := c.load()
	if len(apps2) != 1 {
		t.Fatalf("saveRow changed row count: %d", len(apps2))
	}
}

func TestCacheEmptyLoad(t *testing.T) {
	c, done := testCache(t)
	defer done()
	apps, at, err := c.load()
	if err != nil || len(apps) != 0 || !at.IsZero() {
		t.Fatalf("empty cache load: %d apps, at=%v, err=%v", len(apps), at, err)
	}
}
