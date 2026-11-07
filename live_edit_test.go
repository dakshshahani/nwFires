//go:build live

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"io"
	"testing"
)

const testApplicantID = "zTestNwFiresDoNotUse"



func setupDoc(t *testing.T) {
	t.Helper()
	tok, err := getAccessToken()
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]any{"fields": map[string]any{
		"_id":      map[string]any{"stringValue": testApplicantID},
		"basicInfo": map[string]any{"mapValue": map[string]any{"fields": map[string]any{
			"email":          map[string]any{"stringValue": "nwfires-test@example.com"},
			"legalFirstName": map[string]any{"stringValue": "Test"},
			"legalLastName":  map[string]any{"stringValue": "Applicant"},
			"ageByHackathon": map[string]any{"mapValue": map[string]any{"fields": map[string]any{"HackCamp2026": map[string]any{"integerValue": "20"}}}},
			"skillsList":     map[string]any{"arrayValue": map[string]any{"values": []any{map[string]any{"stringValue": "go"}, map[string]any{"stringValue": "swift"}}}},
		}}},
		"status": map[string]any{"mapValue": map[string]any{"fields": map[string]any{"applicationStatus": map[string]any{"stringValue": "applied"}}}},
	}})
	req, _ := http.NewRequest("POST", basePath+"/"+applicantsCollection+"?documentId="+testApplicantID, strings.NewReader(string(payload)))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("setup %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}

func teardownDoc(t *testing.T) {
	t.Helper()
	tok, _ := getAccessToken()
	req, _ := http.NewRequest("DELETE", basePath+"/"+applicantsCollection+"/"+testApplicantID, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Log("teardown err:", err)
	}
	resp.Body.Close()
}

func TestLiveEditPreferredName(t *testing.T) {
	setupDoc(t)
	defer teardownDoc(t)

	apps, err := fetchApplicants()
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	var a *Applicant
	for i := range apps {
		if apps[i].ID == testApplicantID {
			a = &apps[i]
		}
	}
	if a == nil {
		t.Fatal("test applicant not found in fetch")
	}

	// 1. edit preferred name like the UI does
	if !setLeaf(a.valueTree(), "basicInfo.preferredName", "NwFiresTest") {
		t.Fatal("setLeaf returned false — UI would say 'Cannot edit'")
	}
	if err := saveApplicant(a, []string{"basicInfo"}); err != nil {
		t.Fatalf("saveApplicant: %v", err)
	}

	// 2. refetch and confirm the value persisted and doc intact
	apps2, err := fetchApplicants()
	if err != nil {
		t.Fatalf("refetch: %v", err)
	}
	var b *Applicant
	for i := range apps2 {
		if apps2[i].ID == testApplicantID {
			b = &apps2[i]
		}
	}
	if b == nil {
		t.Fatal("applicant disappeared after save")
	}
	got, ok := getLeaf(b.valueTree(), "basicInfo.preferredName")
	if !ok || got != "NwFiresTest" {
		t.Fatalf("preferredName not persisted: %q", fmt.Sprint(got))
	}
	if _, ok := getLeaf(b.valueTree(), "basicInfo.ageByHackathon.HackCamp2026"); !ok {
		t.Fatalf("integerValue lost in round trip")
	}
	arr, _ := getLeaf(b.valueTree(), "basicInfo.skillsList")
	if al, ok := arr.([]any); !ok || len(al) != 2 {
		t.Fatalf("array lost in round trip: %v", fmt.Sprint(arr))
	}
}

