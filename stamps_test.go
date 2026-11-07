package main

import (
	"encoding/json"
	"testing"
)

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
