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

## Tests

```sh
go test ./...                  # offline suite (latency bench, cache, toast)
go test -tags live ./...       # full lifecycle against prod Firestore
```

The `live` suite drives the real TUI flow against production using a
disposable applicant (`zTestNwFiresDoNotUse`): create → search → edit
preferred name → verify the write → delete.
