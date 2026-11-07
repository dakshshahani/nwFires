package main

// ============================================================
//    ⚙  USER-EDITABLE SETTINGS — edit the two lines below
// ============================================================

// hackathon is the one hackathon nwFires points at.
// It must match the Firestore document ID under the Hackathons collection.
//
//	Known slugs: HackCamp2026, HackCamp2025, nwHacks2026, cmd-f2026, …
//
// To switch hackathons: change this value, then run  nwfires --build .
// The SQLite snapshot is per-hackathon, so nothing bleeds across events.
const hackathon = "HackCamp2026"

// editsWriteToProd is the safety switch for writes: when false, saves refuse
// to touch Firestore prod and surface an error instead. Set to true (default)
// to allow editing.
const editsWriteToProd = true

// ============================================================
//    ⚙  END OF USER-EDITABLE SETTINGS
// ============================================================

// applicantCollection is the Firestore collection path of the selected
// hackathon's applicant roster.
func applicantCollection() string {
	return "Hackathons/" + hackathon + "/Applicants"
}
