package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// ---------- palette ----------

// One accent, used sparingly: purple marks identity and focus, never fills the
// screen. Everything else stays neutral so dense tables remain readable.
var (
	cLipstick = lipgloss.Color("213") // primary accent: brand, selection, focus
	cGrape    = lipgloss.Color("141") // secondary accent: section marks, hints
	cInk      = lipgloss.Color("237") // selection tint
	cPaper    = lipgloss.Color("230") // primary text
	cSlate    = lipgloss.Color("245") // secondary text
	cFog      = lipgloss.Color("241") // labels / tertiary text
	cRule     = lipgloss.Color("238") // hairlines
	cFresh    = lipgloss.Color("42")  // green
	cAlert    = lipgloss.Color("203") // red
	cSunny    = lipgloss.Color("214") // yellow/orange
	cSky      = lipgloss.Color("38")  // blue
)

// gutter is the shared page margin, so every mode lines up on the same edge.
const gutter = 2

var (
	styleBrand   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("16")).Background(cLipstick).Padding(0, 1)
	styleTitle   = lipgloss.NewStyle().Bold(true).Foreground(cPaper)
	styleRule    = lipgloss.NewStyle().Foreground(cRule)
	styleLabel   = lipgloss.NewStyle().Foreground(cFog)
	styleCount   = lipgloss.NewStyle().Foreground(cSlate)
	styleSearch  = lipgloss.NewStyle().Padding(0, 1).Border(lipgloss.NormalBorder(), false, false, true, false).BorderForeground(cLipstick)
	styleRow     = lipgloss.NewStyle().Padding(0, 1)
	styleRowSel  = lipgloss.NewStyle().Padding(0, 1).Background(cInk)
	styleAccent  = lipgloss.NewStyle().Foreground(cLipstick)
	styleName    = lipgloss.NewStyle().Foreground(cPaper)
	styleNameSel = lipgloss.NewStyle().Bold(true).Foreground(cPaper)
	styleEmail   = lipgloss.NewStyle().Foreground(cSlate)
	styleEmailSel = lipgloss.NewStyle().Foreground(lipgloss.Color("252")) // secondary text that survives the selection bar
	styleUID     = lipgloss.NewStyle().Foreground(cFog)
	styleUIDSel  = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	styleDetail  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cRule).Padding(0, 2)
	styleKey     = lipgloss.NewStyle().Foreground(cSlate)
	styleSecHead = lipgloss.NewStyle().Bold(true).Foreground(cGrape)
	styleLeafVal = lipgloss.NewStyle().Foreground(cPaper)
	styleEmpty   = lipgloss.NewStyle().Foreground(cFog)
	styleHintKey = lipgloss.NewStyle().Bold(true).Foreground(cGrape)
	styleHintTxt = lipgloss.NewStyle().Foreground(cSlate)
	styleOK      = lipgloss.NewStyle().Foreground(cFresh).Bold(true)
	styleErr     = lipgloss.NewStyle().Foreground(cAlert).Bold(true)
	styleSaving  = lipgloss.NewStyle().Foreground(cSunny)
)

// statusMeta is the display form of an applicationStatus: a coloured glyph, a
// short human label (raw camelCase never reaches the screen), and its colour.
type statusMeta struct {
	icon  string
	col   lipgloss.Color
	label string
}

var statusColors = map[string]statusMeta{
	"accepted":             {"✓", cFresh, "accepted"},
	"acceptedAndAttending": {"★", cFresh, "attending"},
	"applied":              {"○", cSky, "applied"},     // hollow: just started
	"submitted":            {"●", cSky, "submitted"},   // filled: form complete
	"inReview":             {"◐", cGrape, "in review"}, // half-filled: in process
	"interviewing":         {"◆", cSunny, "interviewing"},
	"waitlisted":           {"—", cSunny, "waitlisted"},
	"rejected":             {"✗", cAlert, "rejected"},
	"withdrawn":            {"⊘", cFog, "withdrawn"}, // an opt-out, not a verdict
	"inProgress":           {"◌", cFog, "in progress"},
}

func statusMetaFor(status string) statusMeta {
	if s, ok := statusColors[status]; ok {
		return s
	}
	if status == "" {
		return statusMeta{"◌", cFog, "no status"}
	}
	// Unknown statuses still get readable casing rather than raw camelCase.
	label := humanizeKey(status)
	if label != "" {
		label = strings.ToLower(label[:1]) + label[1:]
	}
	return statusMeta{"◌", cFog, label}
}

// statusChip is the expressive details-view form: glyph and label share the
// status colour so the state reads at a glance.
func statusChip(status string) string {
	s := statusMetaFor(status)
	return lipgloss.NewStyle().Foreground(s.col).Bold(true).Render(s.icon) +
		lipgloss.NewStyle().Foreground(s.col).Render(" "+s.label)
}

// statusCell is the quiet list form: only the glyph carries colour, the label
// stays secondary so a full page of rows does not turn into confetti. On the
// selection bar the label lifts a step so it stays legible.
func statusCell(status string, width int, sel bool) string {
	s := statusMetaFor(status)
	labelW := width - 2
	if labelW < 1 {
		labelW = 1
	}
	labelStyle := styleEmail
	if sel {
		labelStyle = styleEmailSel
	}
	// Deliberate abbreviations for the narrow column: an ellipsis beside a
	// generic glyph loses the state entirely.
	if labelW <= 9 {
		switch s.label {
		case "interviewing":
			s.label = "interview"
		case "waitlisted":
			s.label = "waitlist"
		case "in progress":
			s.label = "in prog"
		}
	}
	return lipgloss.NewStyle().Foreground(s.col).Render(s.icon) +
		labelStyle.Render(" "+pad(s.label, labelW))
}

type mode int

const (
	modeLoading mode = iota
	modeSearch
	modeDetails
	modeEditPick
	modeEditInput
)

type loadDoneMsg struct {
	apps []Applicant
	err  error
}

type savedMsg struct{ err error }

// cacheLoadedMsg: the boot snapshot came from the local SQLite cache.
type cacheLoadedMsg struct {
	apps []Applicant
	at   time.Time
}

// refreshDoneMsg: the background live fetch finished and replaced the data.
type refreshDoneMsg struct {
	apps []Applicant
	err  error
}

// notifHideMsg dismisses the bottom-right snapshot notification.
type notifHideMsg struct{}

type model struct {
	mode    mode
	err     error
	apps    []Applicant // all
	results []Applicant // filtered

	query   textinput.Model
	cursor  int
	current *Applicant // selected applicant

	detailViewport viewport.Model
	detailContent  string

	editPaths []string
	editIdx   int
	editInput textinput.Model
	editTop   string // top-level field being saved
	statusMsg string
	statusErr bool

	spinner spinner.Model

	// SQLite snapshot state.
	cached         time.Time // when the boot snapshot was taken (zero = live load)
	refreshingNote string    // subtle header note while the bg refresh runs
	notifExpire    time.Time // bottom-right toast expiry (zero = hidden)

	width  int
	height int
}

func newModel() model {
	q := textinput.New()
	q.Placeholder = "Search by name, email, or UID"
	q.PlaceholderStyle = lipgloss.NewStyle().Foreground(cFog)
	q.Focus()
	q.Cursor.Style = lipgloss.NewStyle().Foreground(cLipstick)
	q.TextStyle = lipgloss.NewStyle().Foreground(cPaper).Bold(true)
	q.PromptStyle = lipgloss.NewStyle().Foreground(cLipstick).Bold(true)
	q.Prompt = "⌕ "
	q.Width = 64
	q.CharLimit = 64

	e := textinput.New()
	e.Width = 64
	e.CharLimit = 0
	e.Prompt = "✎ "
	e.PromptStyle = lipgloss.NewStyle().Foreground(cLipstick).Bold(true)
	e.Cursor.Style = lipgloss.NewStyle().Foreground(cLipstick)
	e.TextStyle = lipgloss.NewStyle().Foreground(cPaper).Bold(true)

	vp := viewport.New(80, 20)

	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = lipgloss.NewStyle().Foreground(cLipstick).Bold(true)

	m := model{
		mode:           modeLoading,
		width:          80,
		height:         24,
		query:          q,
		editInput:      e,
		detailViewport: vp,
		spinner:        sp,
	}
	m.resize()
	return m
}

func (m *model) filter() {
	q := strings.TrimSpace(m.query.Value())
	if q == "" {
		// Empty query: point at the master slice, no scan, no allocation.
		m.results = m.apps
		m.cursor = 0
		return
	}
	results := m.results
	if len(results) > 0 && len(m.apps) > 0 && &results[0] == &m.apps[0] {
		// results aliases the master slice; never filter into it.
		results = make([]Applicant, 0, 64)
	} else if cap(results) > 0 {
		results = results[:0]
	} else {
		results = make([]Applicant, 0, 64)
	}
	for _, a := range m.apps {
		if a.matches(q) {
			results = append(results, a)
		}
	}
	m.results = results
	m.cursor = 0
}

// activeCache is the boot SQLite snapshot; nil means the cache is disabled
// (open failed) and the app falls back to live-only behavior.
var activeCache *cache

func (m model) Init() tea.Cmd {
	return tea.Batch(loadBootCmd, m.spinner.Tick)
}

// loadBootCmd opens the SQLite snapshot. Cold cache: fetch, store, and enter
// the UI as before. Warm cache: enter instantly, then refresh in background.
func loadBootCmd() tea.Msg {
	c, err := openCache()
	if err != nil {
		return fetchCmd() // cache unavailable: behave as before
	}
	activeCache = c
	apps, at, lerr := c.load()
	if lerr != nil || len(apps) == 0 {
		msg := fetchCmd() // cold cache: synchronous first fill
		if ld, ok := msg.(loadDoneMsg); ok && ld.err == nil {
			c.store(ld.apps)
		}
		return msg
	}
	return cacheLoadedMsg{apps: apps, at: at}
}

// fetchCmd is the live-only network fetch (also the cold-cache fallback).
func fetchCmd() tea.Msg {
	apps, err := fetchApplicants()
	return loadDoneMsg{apps: apps, err: err}
}

// refreshNow refetches the live roster and replaces both memory and snapshot.
func refreshNow() tea.Msg {
	if activeCache == nil {
		return refreshDoneMsg{err: fmt.Errorf("snapshot unavailable")}
	}
	apps, err := fetchApplicants()
	if err != nil {
		return refreshDoneMsg{err: err}
	}
	activeCache.store(apps)
	return refreshDoneMsg{apps: apps}
}

func (m model) countSuffix() string {
	if m.cached.IsZero() {
		return ""
	}
	return " · snapshot " + approxAge(m.cached)
}

func approxAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < 2*time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loadDoneMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.apps = msg.apps
			m.filter()
			m.mode = modeSearch
			m.cached = time.Now() // first live fill was just stored to the snapshot
		}
		return m, nil

	case cacheLoadedMsg:
		// Instant open on the SQLite snapshot, then refresh from Firestore
		// in the background.
		m.apps = msg.apps
		m.filter()
		m.mode = modeSearch
		m.cached = msg.at
		m.refreshingNote = styleAccent.Render("⟳ refreshing…")
		return m, func() tea.Msg { return refreshNow() }

	case refreshDoneMsg:
		if msg.err != nil {
			// Keep serving the snapshot; surface why the refresh failed.
			m.refreshingNote = ""
			m.statusMsg = " ⟳ Refresh failed: " + msg.err.Error()
			m.statusErr = true
		} else {
			m.apps = msg.apps
			m.filter()
			m.cached = time.Now()
			m.refreshingNote = ""
			m.statusMsg = fmt.Sprintf(" ⟳ Refreshed · %d applicants", len(msg.apps))
			m.statusErr = false
			// Tiny toast in the bottom-right: "latest snapshot fetched",
			// fading away after a few seconds.
			m.notifExpire = time.Now().Add(4 * time.Second)
		}
		return m, tea.Tick(4*time.Second, func(t time.Time) tea.Msg {
			return notifHideMsg{}
		})

	case savedMsg:
		if msg.err != nil {
			m.statusMsg = " ✗ Save failed: " + msg.err.Error()
			m.statusErr = true
		} else {
			m.openDetails(m.current) // re-render from fresh data
			// Set after openDetails, which resets the status slot: otherwise the
			// confirmation is cleared the instant it is produced.
			m.statusMsg = " ✓ Saved"
			m.statusErr = false
		}
		if msg.err == nil && activeCache != nil && m.current != nil {
			// Persist the edit into the snapshot so the cache never resurrects
			// a stale value before the next full refresh.
			a := m.current
			return m, func() tea.Msg {
				_ = activeCache.saveRow(a)
				return nil
			}
		}
		return m, nil

	case notifHideMsg:
		// Expire the toast once its window has passed.
		if !m.notifExpire.IsZero() && !time.Now().Before(m.notifExpire) {
			m.notifExpire = time.Time{}
		}
		return m, nil

	case spinner.TickMsg:
		if m.mode != modeLoading {
			// Once loaded the spinner is gone; stop the tick chain instead of
			// re-queuing renders every 100ms for the rest of the session.
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, tea.Batch(cmd, m.spinner.Tick)

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	switch m.mode {
	case modeLoading:
		return m, nil
	case modeSearch:
		return m.updateSearch(msg)
	case modeDetails:
		return m.updateDetails(msg)
	case modeEditPick:
		return m.updateEditPick(msg)
	case modeEditInput:
		return m.updateEditInput(msg)
	}
	return m, nil
}

func (m model) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "tab":
			if m.cursor < len(m.results)-1 {
				m.cursor++
			}
			return m, nil
		case "enter":
			if len(m.results) > 0 {
				a := m.results[m.cursor]
				m.openDetails(&a)
			}
			return m, nil
		case "esc":
			return m, tea.Quit
		}
	}
	// Only actual typing changes the result set; blink, tick and window
	// messages must not rescan/filter the whole collection or repaint it.
	if _, ok := msg.(tea.KeyMsg); !ok && m.mode == modeSearch && len(m.apps) > 0 {
		return m, nil
	}
	var cmd tea.Cmd
	m.query, cmd = m.query.Update(msg)
	m.filter()
	return m, cmd
}

func (m *model) openDetails(a *Applicant) {
	m.current = a
	m.detailContent = renderDetails(a, m.detailViewport.Width)
	m.detailViewport.SetContent(m.detailContent)
	m.detailViewport.GotoTop()
	m.mode = modeDetails
	m.statusMsg = ""
}

// ---------- details ----------

type detailEntry struct {
	key   string // section-relative ("preferredName", "ageByHackathon.HackCamp2026")
	value string
	meta  bool // arrays and empties read as annotations, not values
}

func collectEntries(v any, prefix string, out *[]detailEntry) {
	switch t := v.(type) {
	case map[string]any:
		for _, k := range sortedKeys(t) {
			p := k
			if prefix != "" {
				p = prefix + "." + k
			}
			collectEntries(t[k], p, out)
		}
	case []any:
		*out = append(*out, detailEntry{prefix, fmt.Sprintf("%d entries", len(t)), true})
	case nil:
		*out = append(*out, detailEntry{prefix, "—", true})
	default:
		val := fmt.Sprint(t)
		if val == "" {
			*out = append(*out, detailEntry{prefix, "—", true})
			return
		}
		*out = append(*out, detailEntry{prefix, val, false})
	}
}

// renderDetails renders one applicant for the scrolling detail viewport.
// width is the usable width inside the card (border and padding excluded).
func renderDetails(a *Applicant, width int) string {
	if width < 10 {
		width = 10
	}
	tree := a.valueTree()

	type section struct {
		name    string
		entries []detailEntry
	}
	var sections []section
	keyW := 0
	for _, name := range sortedKeys(tree) {
		if name == "_id" {
			continue
		}
		var entries []detailEntry
		collectEntries(tree[name], "", &entries)
		if len(entries) == 0 {
			continue
		}
		sections = append(sections, section{name, entries})
		for _, e := range entries {
			if w := lipgloss.Width(e.key); w > keyW {
				keyW = w
			}
		}
	}
	// One aligned value column across the whole document, capped so the value
	// side always keeps room.
	limit := width - 14
	if limit < 8 {
		limit = 8
	}
	if keyW > limit {
		keyW = limit
	}
	if keyW > 28 {
		keyW = 28
	}

	var b strings.Builder

	head := lipgloss.NewStyle().Bold(true).Foreground(cPaper).Render(displayName(a))
	chip := statusChip(a.status())
	if gap := width - lipgloss.Width(head) - lipgloss.Width(chip); gap >= 2 {
		b.WriteString(head + strings.Repeat(" ", gap) + chip + "\n")
	} else {
		b.WriteString(head + "\n" + chip + "\n")
	}

	email := styleEmpty.Render("no email")
	if v := a.email(); v != "" {
		email = styleEmail.Render(v)
	}
	id := styleUID.Render(a.ID)
	dot := styleRule.Render("   ·   ")
	if lipgloss.Width(email)+lipgloss.Width(dot)+lipgloss.Width(id) <= width {
		b.WriteString(email + dot + id + "\n")
	} else {
		b.WriteString(lipgloss.NewStyle().Width(width).Render(email) + "\n")
		b.WriteString(id + "\n")
	}
	b.WriteString(styleRule.Render(strings.Repeat("─", width)) + "\n\n") // one beat after the identity block

	for i, s := range sections {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(styleSecHead.Render("▸ "+humanizeKey(s.name)) + "\n")
		for _, e := range s.entries {
			b.WriteString(renderEntry(e, keyW, width) + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func renderEntry(e detailEntry, keyW, width int) string {
	val := styleLeafVal
	if e.meta {
		val = styleEmpty
	}
	if e.key == "" {
		return "  " + val.MaxWidth(width-2).Render(e.value)
	}
	gap := keyW - lipgloss.Width(e.key) + 2
	if gap < 2 {
		gap = 2
	}
	prefix := "  " + styleKey.Render(e.key) + strings.Repeat(" ", gap)
	valueW := width - lipgloss.Width(prefix)
	if valueW < 10 {
		valueW = 10
	}
	// Wrap long values inside the value column so they keep their indent
	// instead of being clipped at the card edge.
	block := strings.Split(val.Width(valueW).Render(e.value), "\n")
	cont := strings.Repeat(" ", lipgloss.Width(prefix))
	var b strings.Builder
	for i, line := range block {
		if i > 0 {
			b.WriteString("\n" + cont)
		}
		b.WriteString(line)
	}
	return prefix + b.String()
}

// humanizeKey turns stored field names into headings: "basicInfo" -> "Basic info".
func humanizeKey(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '_' || c == '-':
			b.WriteByte(' ')
		case i == 0 && c >= 'a' && c <= 'z':
			b.WriteByte(c - 'a' + 'A')
		case i > 0 && c >= 'A' && c <= 'Z':
			b.WriteByte(' ')
			b.WriteByte(c - 'A' + 'a')
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

func (m model) updateDetails(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc", "left", "backspace":
			m.mode = modeSearch
			return m, nil
		case "up", "k":
			m.detailViewport.LineUp(1)
			return m, nil
		case "down", "j":
			m.detailViewport.LineDown(1)
			return m, nil
		case "pgup":
			m.detailViewport.HalfPageUp()
			return m, nil
		case "pgdown":
			m.detailViewport.HalfPageDown()
			return m, nil
		case "e", "enter":
			return m.startEdit()
		}
	}
	var cmd tea.Cmd
	m.detailViewport, cmd = m.detailViewport.Update(msg)
	return m, cmd
}

func (m model) startEdit() (tea.Model, tea.Cmd) {
	if m.current == nil {
		return m, nil
	}
	tree := m.current.valueTree()

	// Ensure commonly-edited fields exist even when absent from the document,
	// so they can be added (e.g. preferredName on older applicants).
	seed := []string{
		"basicInfo.preferredName",
		"basicInfo.email",
		"basicInfo.legalFirstName",
		"basicInfo.legalLastName",
		"basicInfo.phoneNumber",
		"basicInfo.school",
		"basicInfo.pronouns",
		"status.applicationStatus",
	}
	for _, p := range seed {
		if _, ok := getLeaf(tree, p); !ok {
			setLeaf(tree, p, "")
		}
	}

	m.editPaths = nil
	leafPaths(tree, "", &m.editPaths)
	if len(m.editPaths) == 0 {
		m.statusMsg = " ✗ No editable fields"
		m.statusErr = true
		return m, nil
	}
	m.editIdx = 0
	m.mode = modeEditPick
	return m, nil
}

func (m model) updateEditPick(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			m.mode = modeDetails
			return m, nil
		case "up":
			if m.editIdx > 0 {
				m.editIdx--
			}
			return m, nil
		case "down", "tab":
			if m.editIdx < len(m.editPaths)-1 {
				m.editIdx++
			}
			return m, nil
		case "enter":
			path := m.editPaths[m.editIdx]
			val, ok := getLeaf(m.current.valueTree(), path)
			if !ok || val == nil {
				val = ""
			}
			if _, isMap := val.(map[string]any); isMap {
				return m, nil // not a leaf; ignore
			}
			m.editInput.SetValue(leafToEditable(val))
			m.editInput.Focus()
			m.editInput.CursorEnd()
			m.editTop = topFieldOfPath(path)
			m.mode = modeEditInput
			return m, nil
		}
	}
	return m, nil
}

func (m model) updateEditInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			m.editInput.Blur()
			m.mode = modeEditPick
			return m, nil
		case "enter":
			path := m.editPaths[m.editIdx]
			old, _ := getLeaf(m.current.valueTree(), path)
			newVal := parseEditInput(old, m.editInput.Value())
			tree := m.current.valueTree()
			if !setLeaf(tree, path, newVal) {
				m.statusMsg = " ✗ Cannot edit " + path
				m.statusErr = true
				return m, nil
			}
			m.mode = modeDetails // optimistic; status shown after cmd lands
			m.statusMsg = " ⟳ Saving " + path + "…"
			m.statusErr = false
			a := m.current
			top := m.editTop
			return m, func() tea.Msg {
				err := saveApplicant(a, []string{top})
				return savedMsg{err: err}
			}
		}
	}
	var cmd tea.Cmd
	m.editInput, cmd = m.editInput.Update(msg)
	return m, cmd
}

func displayName(a *Applicant) string {
	if a == nil {
		return "(unknown applicant)"
	}
	n := strings.TrimSpace(a.preferredName() + " " + a.lastName())
	if n == "" {
		return "(unknown applicant)"
	}
	return n
}

func displayNameVal(a Applicant) string { return displayName(&a) }

// ---------- layout ----------

// contentWidth is the usable page width once the gutters are removed.
func (m model) contentWidth() int {
	w := m.width - gutter*2
	if w < 16 {
		w = 16
	}
	return w
}

// bodyHeight is the row budget between the header rule and the footer.
func (m model) bodyHeight() int {
	h := m.height - 5 // header, rule, gap, status slot, footer
	if h < 1 {
		h = 1
	}
	return h
}

func (m *model) resize() {
	if m.width < 20 {
		m.width = 20
	}
	if m.height < 8 {
		m.height = 8
	}
	m.query.Width = m.fieldWidth()
	m.editInput.Width = m.fieldWidth()
	m.detailViewport.Width = m.contentWidth() - 6
	if m.detailViewport.Width < 8 {
		m.detailViewport.Width = 8
	}
	m.detailViewport.Height = m.bodyHeight() - 3 // -1 extra for the scroll-hint line
	if m.detailViewport.Height < 1 {
		m.detailViewport.Height = 1
	}
	if m.current != nil {
		m.detailContent = renderDetails(m.current, m.detailViewport.Width)
		m.detailViewport.SetContent(m.detailContent)
	}
}

// fieldWidth is the text area of a full-width input, leaving room for the
// prompt, the caret and the field's own padding.
func (m model) fieldWidth() int {
	w := m.contentWidth() - 8
	if w < 12 {
		w = 12
	}
	if w > 96 {
		w = 96
	}
	return w
}

// listColumns distributes the row table, reserving a two-column gutter
// between columns. UID and then STATUS drop out on narrow terminals; both
// stay available in the details view.
func (m model) listColumns() (nameW, emailW, uidW, statusW int) {
	inner := m.contentWidth() - 4 // selection marker (2) + row padding (2)
	statusW = 14
	if inner < 52 {
		statusW = 11
	}
	if inner < 40 {
		statusW = 0
	}
	if inner >= 104 {
		uidW = 12
	}
	rest := inner - 2 // between name and email
	if statusW > 0 {
		rest -= statusW + 2
	}
	if uidW > 0 {
		rest -= uidW + 2
	}
	if rest < 10 {
		rest = 10
	}
	nameW = rest * 45 / 100
	emailW = rest - nameW
	return
}

func (m model) editColumns() (pathW, valW int) {
	inner := m.contentWidth() - 4
	pathW = inner * 45 / 100
	if pathW < 6 {
		pathW = 6
	}
	valW = inner - pathW - 2
	if valW < 4 {
		valW = 4
		pathW = inner - valW - 2
		if pathW < 1 {
			pathW = 1
		}
	}
	return
}

// ---------- view rendering ----------

func (m model) View() string {
	if m.mode == modeLoading {
		return m.loadingView()
	}
	return strings.Join([]string{
		indent(m.headerLine()),
		indent(styleRule.Render(strings.Repeat("─", m.contentWidth()))),
		"",
		indent(m.body(m.bodyHeight())),
		indent(m.statusLine()), // always reserved, so saves never shift the footer
		indent(m.footerLine()),
	}, "\n")
}

func (m model) body(h int) string {
	var lines []string
	switch m.mode {
	case modeSearch:
		lines = append(lines, m.field(m.query.View()), "", m.listHeader())
		lines = append(lines, m.listLines(h-4)...)
	case modeDetails:
		lines = append(lines, m.detailCard())
	case modeEditPick:
		lines = append(lines, m.editHeader())
		lines = append(lines, m.editLines(h-1)...)
	case modeEditInput:
		lines = append(lines,
			" "+styleLabel.Render("field")+"  "+styleKey.Render(m.currentEditPath()),
			"",
			m.field(m.editInput.View()),
		)
		// The value being replaced, so the edit is not done blind —
		// the old value was possibly truncated one screen ago.
		cur, _ := getLeaf(m.current.valueTree(), m.currentEditPath())
		shown := leafToEditable(cur)
		if shown == "" {
			shown = "—"
		}
		lines = append(lines,
			" "+styleLabel.Render("current")+"  "+styleEmail.Render(strings.Split(strings.TrimSpace(shown), "\n")[0]),
		)
	}
	return fit(lines, h)
}

func (m model) headerLine() string {
	brand := styleBrand.Render("nwFires")
	left := brand
	if lipgloss.Width(brand)+2+lipgloss.Width(styleTitle.Render("HackCamp 2026")) <= m.contentWidth() {
		left += "  " + styleTitle.Render("HackCamp 2026")
	}

	crumb := ""
	switch m.mode {
	case modeDetails:
		crumb = styleRule.Render("  /  ") + styleLabel.Render("Details")
	case modeEditPick, modeEditInput:
		crumb = styleRule.Render("  /  ") + styleLabel.Render("Editing")
	}
	if crumb != "" && lipgloss.Width(left)+lipgloss.Width(crumb) <= m.contentWidth() {
		left += crumb
	}

	var right string
	switch m.mode {
	case modeSearch:
		note := ""
		if m.refreshingNote != "" && lipgloss.Width(m.refreshingNote)+30 <= m.contentWidth() {
			note = m.refreshingNote + "  "
		}
		right = note + styleCount.Render(fmt.Sprintf("%d / %d", len(m.results), len(m.apps))+m.countSuffix())
	case modeEditPick:
		right = styleCount.Render(fmt.Sprintf("%d fields", len(m.editPaths)))
	case modeEditInput:
		right = styleLabel.Render(m.currentEditPath())
	}
	if right != "" {
		if gap := m.contentWidth() - lipgloss.Width(left) - lipgloss.Width(right); gap >= 4 {
			left += strings.Repeat(" ", gap) + right
		}
	}
	return left
}

func (m model) field(view string) string {
	inner := m.contentWidth() - 2
	if inner < 8 {
		inner = 8
	}
	view = lipgloss.NewStyle().MaxWidth(inner).Render(view)
	return styleSearch.Width(m.contentWidth()).Render(view)
}

func (m model) listHeader() string {
	nameW, emailW, uidW, statusW := m.listColumns()
	line := styleLabel.Render(pad("NAME", nameW)) + "  " + styleLabel.Render(pad("EMAIL", emailW))
	if uidW > 0 {
		line += "  " + styleLabel.Render(pad("UID", uidW))
	}
	if statusW > 0 {
		line += "  " + styleLabel.Render("STATUS")
	}
	return "   " + line
}

func (m model) listLines(size int) []string {
	if size < 1 {
		size = 1
	}
	if len(m.results) == 0 {
		q := strings.TrimSpace(m.query.Value())
		msg := "No applicants loaded."
		if q != "" {
			msg = fmt.Sprintf("Nothing matches %q.", q)
		}
		return []string{"   " + styleEmpty.Render(msg)}
	}
	// Reserve one row for a truncation hint when the list does not fit.
	vis := size
	if len(m.results) > size {
		vis = size - 1
	}
	start, end := window(m.cursor, len(m.results), vis)
	out := make([]string, 0, end-start+1)
	for i := start; i < end; i++ {
		out = append(out, m.renderRow(i))
	}
	if end < len(m.results) {
		out = append(out, "   "+styleEmpty.Render(fmt.Sprintf("↓ %d more", len(m.results)-end)))
	}
	return out
}

func (m model) detailCard() string {
	card := styleDetail.Width(m.contentWidth() - 2).Render(m.detailViewport.View())
	if h := m.scrollHint(); h != "" {
		card += "\n" + lipgloss.NewStyle().Width(m.contentWidth()).Align(lipgloss.Right).Render(h)
	}
	return card
}

// scrollHint reports where you are inside a detail document that overflows
// the card. Empty when everything fits.
func (m model) scrollHint() string {
	contentH := lipgloss.Height(m.detailContent)
	vp := m.detailViewport
	if contentH <= vp.Height {
		return ""
	}
	switch {
	case vp.YOffset == 0:
		return styleEmpty.Render("↓ more")
	case vp.YOffset+vp.Height >= contentH:
		return styleEmpty.Render("↑ end")
	default:
		return styleEmpty.Render("↑↓ more")
	}
}

func (m model) editHeader() string {
	pathW, _ := m.editColumns()
	return "   " + styleLabel.Render(pad("FIELD", pathW)) + "  " + styleLabel.Render("VALUE")
}

func (m model) editLines(size int) []string {
	if size < 1 {
		size = 1
	}
	start, end := window(m.editIdx, len(m.editPaths), size)
	out := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		out = append(out, m.renderEditRow(i))
	}
	return out
}

func (m model) statusLine() string {
	msg := strings.TrimSpace(m.statusMsg)
	if msg == "" {
		return ""
	}
	switch {
	case m.statusErr:
		return styleErr.Render(msg)
	case strings.Contains(msg, "Saving"):
		return styleSaving.Render(msg)
	default:
		return styleOK.Render(msg)
	}
}

func (m model) footerLine() string {
	w := m.contentWidth()
	var hintsPart string
	switch m.mode {
	case modeSearch:
		hintsPart = hints(w, "↑↓", "navigate", "⏎", "view details", "esc", "quit")
	case modeDetails:
		hintsPart = hints(w, "↑↓", "scroll", "⏎", "edit", "esc", "back")
	case modeEditPick:
		hintsPart = hints(w, "↑↓", "choose", "⏎", "edit value", "esc", "cancel")
	case modeEditInput:
		hintsPart = hints(w, "⏎", "save to Firestore prod", "esc", "discard")
	case modeLoading:
		return ""
	}
	if toast := m.toast(); toast != "" {
		gap := w - lipgloss.Width(hintsPart) - lipgloss.Width(toast)
		// Narrow terminals: the toast never fights the hints.
		if gap >= 8 {
			return hintsPart + strings.Repeat(" ", gap) + toast
		}
		return hintsPart
	}
	return hintsPart
}

// toast is the transient bottom-right notification after a snapshot refresh.
func (m model) toast() string {
	if m.notifExpire.IsZero() || time.Now().After(m.notifExpire) {
		return ""
	}
	return styleOK.Render("⟳ latest snapshot fetched") + " "
}

// hints renders "key label" pairs with a quiet gap between groups. The keys
// keep the accent, the verbs stay muted. On narrow terminals it keeps the
// first and last groups (navigate … quit) and drops middles rather than
// spilling past the edge.
func hints(maxW int, kv ...string) string {
	groups := make([]string, 0, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		groups = append(groups, styleHintKey.Render(kv[i])+styleHintTxt.Render(" "+kv[i+1]))
	}
	if len(groups) == 0 {
		return ""
	}
	width := func(gs []string) int {
		w := 0
		for i, g := range gs {
			if i > 0 {
				w += 3
			}
			w += lipgloss.Width(g)
		}
		return w
	}
	out := groups
	if width(out) > maxW && len(groups) > 1 {
		out = []string{groups[0], groups[len(groups)-1]}
		for _, g := range groups[1 : len(groups)-1] {
			cand := append([]string{out[0], g}, out[1:]...)
			if width(cand) <= maxW {
				out = cand
			}
		}
		if width(out) > maxW {
			out = []string{groups[0]}
		}
	}
	res := strings.Join(out, "   ")
	if lipgloss.Width(res) > maxW {
		res = lipgloss.NewStyle().MaxWidth(maxW).Render(res)
	}
	return res
}

func (m model) renderRow(i int) string {
	a := m.results[i]
	nameW, emailW, uidW, statusW := m.listColumns()
	sel := i == m.cursor
	nameStyle, marker, rowStyle := styleName, "  ", styleRow
	emailStyle, uidStyle := styleEmail, styleUID
	if sel {
		nameStyle, marker, rowStyle = styleNameSel, styleAccent.Render("▎")+" ", styleRowSel
		// Secondary fields lift a step on the selection bar so nothing
		// disappears into the 237 tint.
		emailStyle, uidStyle = styleEmailSel, styleUIDSel
	}
	line := nameStyle.Render(pad(displayNameVal(a), nameW)) + "  " + emailStyle.Render(pad(a.email(), emailW))
	if uidW > 0 {
		line += "  " + uidStyle.Render(pad(a.ID, uidW))
	}
	if statusW > 0 {
		line += "  " + statusCell(a.status(), statusW, sel)
	}
	return marker + rowStyle.Width(m.contentWidth()-2).Render(line)
}

func (m model) renderEditRow(i int) string {
	path := m.editPaths[i]
	val, _ := getLeaf(m.current.valueTree(), path)
	disp := leafToEditable(val)
	if disp == "" {
		disp = "—"
	}
	pathW, valW := m.editColumns()
	sel := i == m.editIdx
	rowStyle, marker := styleRow, "  "
	pathStyle, valStyle := styleKey, styleLeafVal
	if sel {
		rowStyle, marker = styleRowSel, styleAccent.Render("▎")+" "
		pathStyle = lipgloss.NewStyle().Bold(true).Foreground(cPaper)
		// The value being edited is the point of this screen: never dim it.
		valStyle = styleLeafVal
	}
	line := pathStyle.Render(elide(path, pathW)) + "  " + valStyle.Render(pad(disp, valW))
	return marker + rowStyle.Width(m.contentWidth()-2).Render(line)
}

func (m model) currentEditPath() string {
	if m.editIdx >= 0 && m.editIdx < len(m.editPaths) {
		return m.editPaths[m.editIdx]
	}
	return ""
}

func (m model) loadingView() string {
	w, h := m.width, m.height
	if w < 20 {
		w = 20
	}
	if h < 8 {
		h = 8
	}
	blockW := w - 8
	if blockW > 64 {
		blockW = 64
	}
	if blockW < 16 {
		blockW = 16
	}

	if m.err != nil {
		msg := lipgloss.NewStyle().Width(blockW).Render(m.err.Error())
		block := strings.Join([]string{
			styleBrand.Render("nwFires"),
			"",
			styleErr.Render("✗ " + msg),
			"",
			styleHintKey.Render("ctrl+c") + styleHintTxt.Render(" quit"),
		}, "\n")
		return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, block)
	}

	status := m.spinner.View() + " " + styleHintTxt.Render("Loading HackCamp 2026 applicants…")
	status = lipgloss.NewStyle().MaxWidth(blockW).Render(status)
	block := strings.Join([]string{
		styleBrand.Render("nwFires"),
		"",
		status,
	}, "\n")
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, block)
}

// ---------- small helpers ----------

// window returns the visible slice bounds for a scrolling windowed list.
func window(idx, total, size int) (int, int) {
	if total <= size {
		return 0, total
	}
	start := idx - size/2
	if start < 0 {
		start = 0
	}
	if start+size > total {
		start = total - size
	}
	return start, start + size
}

func indent(s string) string {
	return lipgloss.NewStyle().PaddingLeft(gutter).Render(s)
}

// fit pins the body to an exact row count so the footer never moves.
func fit(lines []string, h int) string {
	if h < 1 {
		h = 1
	}
	n := 0
	for _, l := range lines {
		n += strings.Count(l, "\n") + 1
	}
	for n < h {
		lines = append(lines, "")
		n++
	}
	split := strings.Split(strings.Join(lines, "\n"), "\n")
	if len(split) > h {
		split = split[:h]
	}
	return strings.Join(split, "\n")
}

func pad(s string, n int) string {
	if n <= 0 {
		return ""
	}
	s = strings.NewReplacer("\n", " ", "\r", " ", "\t", " ").Replace(s)
	// Count display cells, not runes, so CJK/emoji names keep columns aligned.
	if w := ansi.StringWidth(s); w > n {
		if n == 1 {
			return "…"
		}
		return ansi.Truncate(s, n-1, "…")
	}
	return s + strings.Repeat(" ", n-ansi.StringWidth(s))
}

// elide keeps both ends of a long path: field paths are read right-to-left
// (the leaf is the interesting part), so truncating only the tail would hide
// the name of the field being edited.
func elide(s string, n int) string {
	if n <= 0 {
		return ""
	}
	s = strings.NewReplacer("\n", " ", "\r", " ", "\t", " ").Replace(s)
	r := []rune(s)
	if len(r) <= n {
		return s + strings.Repeat(" ", n-len(r))
	}
	if n <= 1 {
		return "…"
	}
	keep := n - 1
	head := keep * 2 / 5
	tail := keep - head
	return string(r[:head]) + "…" + string(r[len(r)-tail:])
}
