package main

import (
	"encoding/json"
	"testing"
)

func TestStampForEvent(t *testing.T) {
	mk := func(hackathon, category any) map[string]any {
		fields := map[string]any{}
		if hackathon != nil {
			fields["hackathon"] = hackathon
		}
		if category != nil {
			fields["category"] = category
		}
		return fields
	}
	// Matched hackathon shows in every case.
	if !stampForEvent(mk("HackCamp2026", "preHackathonWorkshop"), "HackCamp2026") {
		t.Fatal("stamp for HackCamp2026 should be included")
	}
	// Other hackathon, but category "other" → still included.
	if !stampForEvent(mk("nwHacks2025", "other"), "HackCamp2026") {
		t.Fatal(`category "other" stamps must always be included`)
	}
	// Other hackathon, no category → excluded.
	if stampForEvent(mk("nwHacks2025", nil), "HackCamp2026") {
		t.Fatal("stamp for another hackathon should be excluded")
	}
	// No fields at all → excluded (fail closed).
	if stampForEvent(map[string]any{}, "HackCamp2026") {
		t.Fatal("stamp without hackathon/category should be excluded")
	}
}

func TestStampToggleLogic(t *testing.T) {
	s := &Social{ID: "uid-x", Doc: map[string]any{
		"_id": map[string]any{"stringValue": "uid-x"},
		"unlockedStamps": map[string]any{"mapValue": map[string]any{"fields": map[string]any{
			hackathon: map[string]any{"arrayValue": map[string]any{"values": []any{
				map[string]any{"stringValue": "stamp_a"},
			}}},
		}}},
	}}
	if !unlockedFor(s, "stamp_a") {
		t.Fatal("stamp_a should be unlocked")
	}
	if unlockedFor(s, "stamp_b") {
		t.Fatal("stamp_b should be locked")
	}

	// toggle on
	setUnlocked(s, "stamp_b", true)
	if !unlockedFor(s, "stamp_b") {
		t.Fatal("stamp_b should now be unlocked")
	}
	// toggle off stamp_a
	setUnlocked(s, "stamp_a", false)
	if unlockedFor(s, "stamp_a") {
		t.Fatal("stamp_a should now be locked")
	}
	if !unlockedFor(s, "stamp_b") {
		t.Fatal("stamp_b must remain after removing stamp_a")
	}

	// round trip the doc to typed shape like saveSocial does (without network)
	typed, err := goToTyped(s.valueTree())
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(typed)
	_ = b
}
