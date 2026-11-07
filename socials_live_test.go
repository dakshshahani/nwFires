//go:build live

package main

import (
	"net/http"
	"testing"
)

func teardownSocialDoc(t *testing.T) {
	t.Helper()
	tok, _ := getAccessToken()
	req, _ := http.NewRequest("DELETE", basePath+"/Socials/"+testApplicantID, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Log("teardown social err:", err)
	}
	resp.Body.Close()
}

// Socials/{uid} lifecycle for the disposable applicant: absent at first
// (missing=true), first save creates it, second read sees it. Idempotent:
// any pre-existing doc from a previous run is deleted first.
func TestLiveSocialsDoc(t *testing.T) {
	teardownSocialDoc(t)
	setupDoc(t)
	defer teardownDoc(t)
	defer teardownSocialDoc(t)

	s, err := fetchSocial(testApplicantID)
	if err != nil {
		t.Fatal(err)
	}
	if !s.missing {
		t.Fatalf("expected missing doc after cleanup, got %v", s.Doc)
	}
	s.socialDocSeed()
	if !setLeaf(s.valueTree(), "socialLinks.github", "https://github.com/nwfires-test") {
		t.Fatal("could not set socialLinks.github on seed doc")
	}
	if err := saveSocial(s, []string{"socialLinks"}); err != nil {
		t.Fatalf("create via patch: %v", err)
	}

	s2, err := fetchSocial(testApplicantID)
	if err != nil {
		t.Fatal(err)
	}
	if s2.missing {
		t.Fatal("social doc did not persist")
	}
	// _id is stored client-side on read (admin mirrors doc.id), so only the
	// patched field is expected back.
	if v, _ := getLeaf(s2.valueTree(), "socialLinks"); v == nil {
		t.Fatal("socialLinks not written")
	}
}

// The /Stamps collection must load names; uid→name display depends on it.
func TestLiveStampsFetch(t *testing.T) {
	stamps, err := fetchStamps()
	if err != nil {
		t.Fatal(err)
	}
	if len(stamps) == 0 {
		t.Fatal("no stamps")
	}
	named := 0
	for _, s := range stamps {
		if s.Name != "" {
			named++
		}
	}
	t.Logf("stamps: %d total, %d named", len(stamps), named)
	if named == 0 {
		t.Fatal("stamps loaded but all names empty")
	}
}

// Renderer: stamp ids map to "Name" text when the id2name map matches.
func TestStampNameDisplay(t *testing.T) {
	m := model{stampID2name: map[string]string{"abc123": "Devpost Rocket"},
		stampName2id: map[string]string{"Devpost Rocket": "abc123"}}
	got := displayStampValue([]any{"abc123"}, m.stampID2name)
	if got != "Devpost Rocket" {
		t.Fatalf("display: %q", got)
	}
	ids, err := stampIDsFromNames("Devpost Rocket", m.stampName2id)
	if err != nil || len(ids) != 1 || ids[0] != "abc123" {
		t.Fatalf("names→ids: %v %v", ids, err)
	}
	if _, err := stampIDsFromNames("Nope Stamp", m.stampName2id); err == nil {
		t.Fatal("unknown stamp name accepted")
	}
}
