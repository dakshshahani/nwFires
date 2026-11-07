//go:build live

package main

import (
	"testing"
)

// Full live toggle flow for the disposable applicant: set up doc, start the
// picker, toggle a real stamp on then off, verifying each write on a fresh
// read of Socials/{uid}.
func TestLiveStampToggle(t *testing.T) {
	teardownSocialDoc(t)
	setupDoc(t)
	defer teardownDoc(t)
	defer teardownSocialDoc(t)

	stamps, err := fetchStamps()
	if err != nil {
		t.Fatal(err)
	}
	if len(stamps) == 0 {
		t.Fatal("no stamps")
	}
	target := stamps[0]

	// Seed/hydrate the Socials doc for the test applicant.
	s, err := fetchSocial(testApplicantID)
	if err != nil {
		t.Fatal(err)
	}
	if s.missing {
		s.socialDocSeed()
	}

	// ON
	setUnlocked(s, target.ID, true)
	if err := saveSocial(s, []string{"unlockedStamps"}); err != nil {
		t.Fatalf("unlock save: %v", err)
	}
	fresh, err := fetchSocial(testApplicantID)
	if err != nil {
		t.Fatal(err)
	}
	if !unlockedFor(fresh, target.ID) {
		t.Fatalf("stamp %s not unlocked on server", target.Name)
	}

	// OFF
	setUnlocked(fresh, target.ID, false)
	if err := saveSocial(fresh, []string{"unlockedStamps"}); err != nil {
		t.Fatalf("lock save: %v", err)
	}
	fresh2, err := fetchSocial(testApplicantID)
	if err != nil {
		t.Fatal(err)
	}
	if unlockedFor(fresh2, target.ID) {
		t.Fatalf("stamp %s not removed on server", target.Name)
	}
	t.Logf("toggle on/off for %q persisted and reverted", target.Name)
}
