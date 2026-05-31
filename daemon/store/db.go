package store

import (
	"database/sql"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS notebooks (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    is_owner INTEGER DEFAULT 1,
    created_at TEXT,
    url TEXT,
    emoji TEXT DEFAULT '📒',
    cached_at TEXT DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS jobs (
    id TEXT PRIMARY KEY,
    notebook_id TEXT NOT NULL,
    notebook_title TEXT DEFAULT '',
    url TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    tasks TEXT NOT NULL DEFAULT '[]',
    source_ids TEXT DEFAULT '[]',
    task_results TEXT DEFAULT '{}',
    error TEXT DEFAULT '',
    retry_count INTEGER DEFAULT 0,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP,
    completed_at TEXT DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_notebooks_cached_at ON notebooks(cached_at);
`
	_, err := s.db.Exec(schema)
	
	// Migration: add source_ids to existing tables
	_ = s.db.QueryRow("SELECT source_ids FROM jobs LIMIT 1").Scan(new(any)) // Check if it exists
	s.db.Exec("ALTER TABLE jobs ADD COLUMN source_ids TEXT DEFAULT '[]'") // Ignore error if exists

	return err
}
