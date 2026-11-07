package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	project  = "nwplus-ubc"
	basePath = "https://firestore.googleapis.com/v1/projects/nwplus-ubc/databases/(default)/documents"
)

var applicantsCollection = "" // set from config.go at startup

func init() {
	applicantsCollection = applicantCollection()
}

var tokenCache struct {
	token string
	exp   time.Time
}

// getAccessToken returns a Google access token via gcloud ADC, cached ~45min.
func getAccessToken() (string, error) {
	if tokenCache.token != "" && time.Now().Before(tokenCache.exp) {
		return tokenCache.token, nil
	}
	out, err := exec.Command("gcloud", "auth", "application-default", "print-access-token").Output()
	if err != nil {
		return "", fmt.Errorf("gcloud auth failed (run `gcloud auth application-default login`): %w", err)
	}
	tokenCache.token = strings.TrimSpace(string(out))
	tokenCache.exp = time.Now().Add(45 * time.Minute)
	return tokenCache.token, nil
}

// Applicant is one applicant of the configured hackathon (config.go).
type Applicant struct {
	ID     string         // Firestore document ID used as UID
	Doc    map[string]any // typed firestore value tree (raw)
	fields map[string]any // converted Go value tree
	blob   string         // lowercase search haystack (lazy)
	social *Social        // companion Socials/{uid} doc (lazy)
}

func (a *Applicant) valueTree() map[string]any {
	if a.fields == nil {
		a.fields = typedToGo(a.Doc).(map[string]any)
	}
	return a.fields
}

func (a *Applicant) email() string   { v, _ := a.valueTree()["basicInfo"].(map[string]any)["email"].(string); return v }
func (a *Applicant) firstName() string {
	v, _ := a.valueTree()["basicInfo"].(map[string]any)["legalFirstName"].(string); return v }
func (a *Applicant) lastName() string {
	v, _ := a.valueTree()["basicInfo"].(map[string]any)["legalLastName"].(string); return v }
func (a *Applicant) preferredName() string {
	bi, _ := a.valueTree()["basicInfo"].(map[string]any)
	if v, ok := bi["preferredName"].(string); ok && v != "" {
		return v
	}
	return a.firstName()
}
func (a *Applicant) status() string {
	st, _ := a.valueTree()["status"].(map[string]any)
	v, _ := st["applicationStatus"].(string); return v
}

// matches mirrors admin-v2's applicant search: the query is split on whitespace
// and every word must appear (case-insensitive substring) in at least one of
// UID / email / preferred or legal names. The haystack is a single precomputed
// lowercase blob, so typing costs a couple of Contains calls per applicant.
func (a *Applicant) matches(q string) bool {
	words := strings.Fields(strings.ToLower(strings.TrimSpace(q)))
	if len(words) == 0 {
		return true
	}
	blob := a.searchBlob()
	for _, w := range words {
		if !strings.Contains(blob, w) {
			return false
		}
	}
	return true
}

// searchBlob lazily joins the searchable fields into one lowercase string,
// computed at most once per applicant for the whole session.
func (a *Applicant) searchBlob() string {
	if a.blob != "" {
		return a.blob
	}
	bi, _ := a.valueTree()["basicInfo"].(map[string]any)
	parts := []string{a.ID, str(bi["email"]), a.preferredName(), str(bi["legalFirstName"]), str(bi["legalLastName"])}
	a.blob = strings.ToLower(strings.Join(parts, "\x00"))
	return a.blob
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

// fetchApplicants downloads ALL applicants, paged (Firestore REST caps 300/page).
func fetchApplicants() ([]Applicant, error) {
	var out []Applicant
	pageToken := ""
	for {
		url := basePath + "/" + applicantsCollection + "?pageSize=300"
		if pageToken != "" {
			url += "&pageToken=" + pageToken
		}
		tok, err := getAccessToken()
		if err != nil {
			return nil, err
		}
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("fetch %d: %s", resp.StatusCode, truncate(string(body), 300))
		}
		var page struct {
			Documents     []struct {
				Name   string         `json:"name"`
				Fields map[string]any `json:"fields"`
			} `json:"documents"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, err
		}
		for _, d := range page.Documents {
			out = append(out, Applicant{
				ID:     d.Name[strings.LastIndex(d.Name, "/")+1:],
				Doc:    d.Fields,
				fields: nil,
			})
		}
		if page.NextPageToken == "" {
			break
		}
		pageToken = page.NextPageToken
	}
	return out, nil
}

// saveApplicant patches the given top-level field paths (e.g. "basicInfo", "status")
// with the applicant's current (edited) field tree.
func saveApplicant(a *Applicant, fieldPaths []string) error {
	if !editsWriteToProd {
		return fmt.Errorf("writes are disabled (editsWriteToProd = false in config.go)")
	}
	tok, err := getAccessToken()
	if err != nil {
		return err
	}
	url := basePath + "/" + applicantsCollection + "/" + a.ID + "?"
	for _, fp := range fieldPaths {
		url += "updateMask.fieldPaths=" + fp + "&"
	}
	url = strings.TrimSuffix(url, "&")

	typed, err := goToTyped(a.valueTree())
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]any{"fields": typed})
	req, _ := http.NewRequest("PATCH", url, bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("save %d: %s", resp.StatusCode, truncate(string(body), 300))
	}
	var updated struct {
		Fields map[string]any `json:"fields"`
		UpdateTime string     `json:"updateTime"`
	}
	if err := json.Unmarshal(body, &updated); err == nil {
		a.Doc = updated.Fields
		a.fields = nil
	}
	return nil
}

// Social is the companion Socials/{uid} doc for an applicant.
// Doc ID = applicant Firebase auth UID (per portal-v2 services/socials.ts).
type Social struct {
	ID     string         // same as the applicant's UID
	Doc    map[string]any // raw typed fields
	fields map[string]any
	missing bool // no doc exists yet; display-only, created on first write
}

func (s *Social) valueTree() map[string]any {
	if s.fields == nil {
		s.fields = typedToGo(s.Doc).(map[string]any)
	}
	return s.fields
}

// socialDocSeed gives an absent doc the fields the portal-type Social has,
// so a first edit creates something well-formed instead of a bare _id.
func (s *Social) socialDocSeed() {
	if !s.missing || s.Doc != nil {
		return
	}
	s.Doc = map[string]any{
		"_id":         map[string]any{"stringValue": s.ID},
		"email":       map[string]any{"stringValue": ""},
		"socialLinks": map[string]any{"mapValue": map[string]any{"fields": map[string]any{}}},
	}
	s.fields = nil
}

// fetchSocial reads Socials/{uid}; absent docs are reported via missing.
func fetchSocial(uid string) (*Social, error) {
	s := &Social{ID: uid}
	tok, err := getAccessToken()
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequest("GET", basePath+"/Socials/"+uid, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode == 404 {
		s.missing = true
		return s, nil
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("social %s: %d %s", uid, resp.StatusCode, truncate(string(body), 200))
	}
	var doc struct {
		ID     string         `json:"name"`
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	s.Doc = doc.Fields
	return s, nil
}

// saveSocial patches top-level fields of Socials/{uid}, creating the doc if
// the applicant has none yet.
func saveSocial(s *Social, fieldPaths []string) error {
	if !editsWriteToProd {
		return fmt.Errorf("writes are disabled (editsWriteToProd = false in config.go)")
	}
	tok, err := getAccessToken()
	if err != nil {
		return err
	}
	url := basePath + "/Socials/" + s.ID + "?"
	for _, fp := range fieldPaths {
		url += "updateMask.fieldPaths=" + fp + "&"
	}
	url = strings.TrimSuffix(url, "&")

	// Seed the identity fields the portal guarantees on creation.
	tree := s.valueTree()
	if _, ok := tree["_id"]; !ok {
		tree["_id"] = s.ID
	}

	typed, err := goToTyped(tree)
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]any{"fields": typed})
	req, _ := http.NewRequest("PATCH", url, bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("social save %d: %s", resp.StatusCode, truncate(string(body), 300))
	}
	var updated struct {
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal(body, &updated); err == nil {
		s.Doc = updated.Fields
		s.fields = nil
	}
	s.missing = false
	return nil
}

// Stamp pairs a /Stamps doc ID with its display name.
type Stamp struct {
	ID   string
	Name string
}

// fetchStamps reads the /Stamps collection (name lives here, not on the
// applicant — admin-v2/lib/stamps.ts resolves via stamp.name).
func fetchStamps() ([]Stamp, error) {
	tok, err := getAccessToken()
	if err != nil {
		return nil, err
	}
	var out []Stamp
	pageToken := ""
	for {
		url := basePath + "/Stamps?pageSize=300"
		if pageToken != "" {
			url += "&pageToken=" + pageToken
		}
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("stamps %d: %s", resp.StatusCode, truncate(string(body), 200))
		}
		var page struct {
			Documents []struct {
				Name   string         `json:"name"`
				Fields map[string]any `json:"fields"`
			} `json:"documents"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, err
		}
		for _, d := range page.Documents {
			tree := typedToGo(d.Fields).(map[string]any)
			id := d.Name[strings.LastIndex(d.Name, "/")+1:]
			out = append(out, Stamp{ID: id, Name: str(tree["name"])})
		}
		if page.NextPageToken == "" {
			break
		}
		pageToken = page.NextPageToken
	}
	return out, nil
}

// stampName resolves a stamp id to its display name; unknown ids pass through.
func stampName(id string, id2name map[string]string) string {
	if n, ok := id2name[id]; ok {
		return n
	}
	return id
}

// stampIDsFromNames converts a comma list of stamp names back into ids.
// Unknown names fail with an error so stale text can't corrupt the doc.
func stampIDsFromNames(input string, name2id map[string]string) ([]string, error) {
	var ids []string
	for _, p := range strings.Split(input, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id, ok := name2id[p]
		if !ok {
			return nil, fmt.Errorf("unknown stamp %q", p)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// --- display helpers ---



// renderTree flattens a value tree into "path: value" lines for the details view.
func renderTree(v any, prefix string, skip map[string]bool, out *[]string) {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			var p string
			if prefix == "" {
				p = k
			} else {
				p = prefix + "." + k
			}
			if skip[k] && prefix == "" {
				continue
			}
			renderTree(t[k], p, skip, out)
		}
	case []any:
		*out = append(*out, prefix+".count: "+strconv.Itoa(len(t)))
	case nil:
		*out = append(*out, prefix+": (empty)")
	default:
		*out = append(*out, prefix+": "+fmt.Sprint(t))
	}
}

// leafPaths returns scalar leaf paths ("dotted") rooted under the given top-level keys.
// Arrays are treated as non-editable values (joined for display, editable as comma list).
func leafPaths(v any, prefix string, out *[]string) {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			var p string
			if prefix == "" {
				p = k
			} else {
				p = prefix + "." + k
			}
			leafPaths(t[k], p, out)
		}
	default: // scalar or array or nil -> leaf
		if prefix != "" {
			*out = append(*out, prefix)
		}
	}
}

// getLeaf resolves a dotted path to (value, true).
func getLeaf(v map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	cur := v
	for i, p := range parts {
		if i == len(parts)-1 {
			val, ok := cur[p]
			return val, ok
		}
		next, ok := cur[p].(map[string]any)
		if !ok {
			return nil, false
		}
		cur = next
	}
	return nil, false
}

// setLeaf assigns value at a dotted path, creating intermediate maps when
// they do not exist yet (e.g. first unlockedStamps write on a fresh socials
// doc). Returns false only when an intermediate is a non-map value that
// would have to be clobbered.
func setLeaf(v map[string]any, path string, value any) bool {
	parts := strings.Split(path, ".")
	cur := v
	for i, p := range parts {
		if i == len(parts)-1 {
			cur[p] = value
			return true
		}
		next, ok := cur[p].(map[string]any)
		if !ok {
			if cur[p] != nil {
				return false
			}
			next = map[string]any{}
			cur[p] = next
		}
		cur = next
	}
	return false
}

// topFieldOfPath: "basicInfo.phone" -> "basicInfo"; "status" -> "status"
func topFieldOfPath(path string) string {
	return strings.SplitN(path, ".", 2)[0]
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
