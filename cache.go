package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// cache holds a local snapshot of the applicant roster so the TUI opens
// instantly on cached data; a background refresh then swaps in the live copy.
type cache struct {
	db *sql.DB
}

// cachePath is the snapshot location in the user's cache dir. Tests override
// it with NWFIRES_CACHE so they never touch real data.
func cachePath() (string, error) {
	if p := os.Getenv("NWFIRES_CACHE"); p != "" {
		return p, nil
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "nwFires", hackathon+".sqlite"), nil
}

func openCache() (*cache, error) {
	p, err := cachePath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", p+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`
create table if not exists applicants (
  uid  text primary key,
  data text not null  -- JSON snapshot of the raw Firestore field tree
);
create table if not exists meta (
  key   text primary key,
  value text not null
);`); err != nil {
		db.Close()
		return nil, err
	}
	return &cache{db: db}, nil
}

// load returns every cached applicant and the time the snapshot was taken.
// Empty time means the cache is cold.
func (c *cache) load() ([]Applicant, time.Time, error) {
	rows, err := c.db.Query(`select uid, data from applicants`)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer rows.Close()
	var out []Applicant
	for rows.Next() {
		var id, data string
		if err := rows.Scan(&id, &data); err != nil {
			return nil, time.Time{}, err
		}
		var doc map[string]any
		if err := json.Unmarshal([]byte(data), &doc); err != nil {
			return nil, time.Time{}, fmt.Errorf("cache row %s: %w", id, err)
		}
		out = append(out, Applicant{ID: id, Doc: doc, fields: nil})
	}
	if err := rows.Err(); err != nil {
		return nil, time.Time{}, err
	}
	if len(out) == 0 {
		return nil, time.Time{}, nil
	}
	var stamp string
	err = c.db.QueryRow(`select value from meta where key='lastRefreshed'`).Scan(&stamp)
	if err != nil && err != sql.ErrNoRows {
		return nil, time.Time{}, err
	}
	at := time.Time{}
	if err == nil {
		at, _ = time.Parse(time.RFC3339, stamp)
	}
	return out, at, nil
}

// store replaces the whole snapshot in one transaction.
func (c *cache) store(apps []Applicant) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()
	if _, err = tx.Exec(`delete from applicants`); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`insert into applicants (uid, data) values (?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for i := range apps {
		b, err := json.Marshal(apps[i].Doc)
		if err != nil {
			return err
		}
		if _, err = stmt.Exec(apps[i].ID, string(b)); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`insert into meta (key, value) values ('lastRefreshed', ?) on conflict(key) do update set value=excluded.value`, time.Now().Format(time.RFC3339)); err != nil {
		return err
	}
	return tx.Commit()
}

// saveRow persists a single successful edit back into the snapshot so the
// cache does not resurrect stale values before the next full refresh.
func (c *cache) saveRow(a *Applicant) error {
	b, err := json.Marshal(a.Doc)
	if err != nil {
		return err
	}
	_, err = c.db.Exec(`insert into applicants (uid, data) values (?, ?)
		on conflict(uid) do update set data=excluded.data`, a.ID, string(b))
	return err
}

func (c *cache) close() { c.db.Close() }
