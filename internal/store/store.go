// Package store persists praxis state to SQLite. The context files
// written into each harness are the working state; this database is the
// source of truth from which they can be recreated at any point.
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/michael-duren/praxis/internal/domain"
)

// Store wraps the SQLite database holding skills, context entries,
// harness enablement, and settings.
type Store struct {
	db *sql.DB
}

// DefaultPath returns the XDG-style location for the praxis database.
func DefaultPath() (string, error) {
	dir := os.Getenv("XDG_DATA_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dir, "praxis", "praxis.db"), nil
}

// Open opens (creating if needed) the database at path and runs migrations.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS skills (
	id       INTEGER PRIMARY KEY AUTOINCREMENT,
	name     TEXT NOT NULL UNIQUE,
	category TEXT NOT NULL DEFAULT '',
	rank     TEXT NOT NULL DEFAULT 'novice',
	notes    TEXT NOT NULL DEFAULT '',
	updated  TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS context_entries (
	id      INTEGER PRIMARY KEY AUTOINCREMENT,
	repo    TEXT NOT NULL DEFAULT '',
	title   TEXT NOT NULL,
	body    TEXT NOT NULL,
	updated TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS harnesses (
	name    TEXT PRIMARY KEY,
	enabled INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);`)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// --- skills ---

// UpsertSkill inserts or updates a skill by name and returns its ID.
func (s *Store) UpsertSkill(sk domain.UserSkill) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`
INSERT INTO skills (name, category, rank, notes, updated)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(name) DO UPDATE SET
	category = excluded.category,
	rank     = excluded.rank,
	notes    = excluded.notes,
	updated  = excluded.updated`,
		sk.Name, sk.Category, sk.Rank.String(), sk.Notes, now)
	if err != nil {
		return 0, fmt.Errorf("upsert skill %q: %w", sk.Name, err)
	}
	if id, err := res.LastInsertId(); err == nil && id != 0 {
		return id, nil
	}
	var id int64
	err = s.db.QueryRow(`SELECT id FROM skills WHERE name = ?`, sk.Name).Scan(&id)
	return id, err
}

// Skills returns all skills ordered by name.
func (s *Store) Skills() ([]domain.UserSkill, error) {
	rows, err := s.db.Query(`SELECT id, name, category, rank, notes, updated FROM skills ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	defer rows.Close()

	var out []domain.UserSkill
	for rows.Next() {
		var sk domain.UserSkill
		var rank, updated string
		if err := rows.Scan(&sk.ID, &sk.Name, &sk.Category, &rank, &sk.Notes, &updated); err != nil {
			return nil, err
		}
		if sk.Rank, err = domain.ParseRank(rank); err != nil {
			return nil, err
		}
		sk.Updated, _ = time.Parse(time.RFC3339, updated)
		out = append(out, sk)
	}
	return out, rows.Err()
}

// DeleteSkill removes a skill by name.
func (s *Store) DeleteSkill(name string) error {
	_, err := s.db.Exec(`DELETE FROM skills WHERE name = ?`, name)
	return err
}

// --- context entries ---

// UpsertContextEntry inserts (ID zero) or updates (ID set) an entry.
func (s *Store) UpsertContextEntry(e domain.ContextEntry) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if e.ID == 0 {
		res, err := s.db.Exec(`INSERT INTO context_entries (repo, title, body, updated) VALUES (?, ?, ?, ?)`,
			e.Scope.Repo, e.Title, e.Body, now)
		if err != nil {
			return 0, fmt.Errorf("insert context entry: %w", err)
		}
		return res.LastInsertId()
	}
	_, err := s.db.Exec(`UPDATE context_entries SET repo = ?, title = ?, body = ?, updated = ? WHERE id = ?`,
		e.Scope.Repo, e.Title, e.Body, now, e.ID)
	return e.ID, err
}

// ContextEntries returns all entries, global first, then by repo.
func (s *Store) ContextEntries() ([]domain.ContextEntry, error) {
	rows, err := s.db.Query(`SELECT id, repo, title, body, updated FROM context_entries ORDER BY repo, id`)
	if err != nil {
		return nil, fmt.Errorf("list context entries: %w", err)
	}
	defer rows.Close()

	var out []domain.ContextEntry
	for rows.Next() {
		var e domain.ContextEntry
		var updated string
		if err := rows.Scan(&e.ID, &e.Scope.Repo, &e.Title, &e.Body, &updated); err != nil {
			return nil, err
		}
		e.Updated, _ = time.Parse(time.RFC3339, updated)
		out = append(out, e)
	}
	return out, rows.Err()
}

// DeleteContextEntry removes an entry by ID.
func (s *Store) DeleteContextEntry(id int64) error {
	_, err := s.db.Exec(`DELETE FROM context_entries WHERE id = ?`, id)
	return err
}

// --- harnesses ---

// SetHarnessEnabled records whether praxis should write to a harness.
func (s *Store) SetHarnessEnabled(name string, enabled bool) error {
	_, err := s.db.Exec(`
INSERT INTO harnesses (name, enabled) VALUES (?, ?)
ON CONFLICT(name) DO UPDATE SET enabled = excluded.enabled`, name, boolToInt(enabled))
	return err
}

// EnabledHarnesses returns the names of harnesses praxis writes to.
func (s *Store) EnabledHarnesses() (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT name, enabled FROM harnesses`)
	if err != nil {
		return nil, fmt.Errorf("list harnesses: %w", err)
	}
	defer rows.Close()

	out := map[string]bool{}
	for rows.Next() {
		var name string
		var enabled int
		if err := rows.Scan(&name, &enabled); err != nil {
			return nil, err
		}
		out[name] = enabled != 0
	}
	return out, rows.Err()
}

// --- settings ---

const keyAutonomy = "global_autonomy"

// SaveSettings persists praxis-wide settings.
func (s *Store) SaveSettings(set domain.Settings) error {
	_, err := s.db.Exec(`
INSERT INTO settings (key, value) VALUES (?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		keyAutonomy, set.GlobalAutonomy.String())
	return err
}

// LoadSettings returns saved settings, defaulting to guided autonomy.
func (s *Store) LoadSettings() (domain.Settings, error) {
	set := domain.Settings{GlobalAutonomy: domain.AutonomyGuided}
	var val string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, keyAutonomy).Scan(&val)
	if err == sql.ErrNoRows {
		return set, nil
	}
	if err != nil {
		return set, err
	}
	mode, err := domain.ParseAutonomyMode(val)
	if err != nil {
		return set, err
	}
	set.GlobalAutonomy = mode
	return set, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
