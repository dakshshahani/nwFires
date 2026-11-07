# nwFires

Terminal UI for nwPlus UBC's HackCamp 2026 roster on Firebase (Firestore prod).
A Bubble Tea app: type to search applicants by **UID, email or name**, arrow-key
through results, enter to view all their details, and edit any field — writes
go straight to Firestore prod. A local SQLite snapshot makes boots instant and
refetches the latest data in the background.

```
nwfires          # launch the TUI
nwfires --build  # rebuild the binary from source
```

## 🛠 Setup (new machines)

```zsh
./setup.sh           # checks/installs prerequisites, builds binary if missing, sets up the alias
./setup.sh --build   # force a rebuild too
./setup.sh --skip-requirements   # skip the prerequisite checks
```

It checks — and with your confirmation, installs — the prerequisites:
- **Go** (builds the binary)
- **gcloud CLI** (`brew install google-cloud-sdk`)
- **Firebase CLI** (npm or brew)
- **Firebase credentials** — if `gcloud auth application-default` isn't set up, it offers to run `gcloud auth application-default login` (what the TUI reads auth actually uses)

Non-interactive environments skip the prompts; pass `--skip-requirements` to
jump straight to the launcher install.

This installs the `"hackcamp 2026"` script and the `nwfires` wrapper into
`~/bin`, and appends `alias nwfires="$HOME/bin/nwfires"` to `~/.zshrc` if it's
not already there (idempotent — safe to re-run). `source ~/.zshrc` afterwards.
A fresh clone of this repo needs `go` on your PATH plus the Firebase ADC login
from the note in the Quick start.

First run needs `gcloud auth application-default login` (reads via ADC).

---

## ⚙️ Changing the target hackathon (manual — user-editable)

Open **`config.go`** and edit the two flagged lines at the top:

```go
const hackathon = "HackCamp2026"
```

- `hackathon` — the Firestore document ID under the `Hackathons` collection.
  The TUI queries `Hackathons/<hackathon>/Applicants`.
  Known slugs: `HackCamp2026`, `HackCamp2025`, `nwHacks2026`, `cmd-f2026`, …

- `editsWriteToProd` — safety switch. `false` makes saves show an error
  instead of writing to Firebase prod; keep `true` only when you intend to
  edit real data.

After changing either value **rebuild** (the flag is compiled in on purpose,
with no config file to lose):

```sh
nwfires --build
```

Each hackathon keeps its own SQLite snapshot
(`~/Library/Caches/nwFires/<hackathon>.sqlite`), so switching is clean — the
next run with a new slug does one cold fetch, and boots stay instant after.

---

## Layout

| File | Purpose |
|---|---|
| `main.go` | entry point |
| `ui.go` | Bubble Tea model, styles, key handling, views |
| `firebase.go` | Firestore REST fetch/save + applicant search |
| `cache.go` | SQLite snapshot layer (boot cache + bg refresh) |
| `convert.go` | Firestore typed values ⇄ Go values, edit parsing |
| `config.go` | ⚙️ **user-editable flags (hackathon slug, write switch)** |

## Bonus records

- **Socials doc** — selecting an applicant also loads their `Socials/{uid}`
  companion doc and appends all its fields to the details view (nicknamed the
  `◆ Socials` section). Its fields are editable too: paths prefixed `social.`
  in the edit list write to `Socials/{uid}` (patching only the top-level field
  touched). If the doc doesn't exist yet, the first edit creates it in the
  shape the portal expects.
- **Stamps by name** — stamp IDs never appear: the `/Stamps` collection is
  loaded at boot (filtered to stamps whose docs belong to the configured
  hackathon, plus category:"other" stamps — for HackCamp2026 that's 21 of the
  57 docs) and `unlockedStamps` ids resolve to their `name` on display,
  in both the details view and the edit list. When editing an
  `unlockedStamps` field you type stamp *names* (comma-separated); names are
  converted back to stamp IDs before the save, and unknown names are
  rejected so raw ids can't sneak into the doc.
- **Manual stamping** — press `s` in the details view to open the stamp
  picker: the hackathon's stamps (plus category:"other") sorted by name,
  with `✓` on stamps the
  applicant has unlocked and `○` on locked ones. Enter/space toggles the
  highlighted stamp instantly — the change writes to
  `Socials/{uid}.unlockedStamps[<hackathon>]` right away and the row flips
  in place (a no-socials-doc applicant gets one created on first toggle).
  `esc` returns to the details view.

## Tests

```sh
go test ./...                  # offline suite (latency bench, cache, toast)
go test -tags live ./...       # full lifecycle against prod Firestore
```

The `live` suite drives the real TUI flow against production using a
disposable applicant (`zTestNwFiresDoNotUse`): create → search → edit
preferred name → verify the write → delete.
