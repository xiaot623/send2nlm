package store

import (
	"context"
	"database/sql"
	"time"

	"send2nlm/core"
)

func (s *Store) ListNotebooks(ctx context.Context) ([]core.Notebook, *time.Time, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, title, is_owner, created_at, url, emoji, cached_at
FROM notebooks
ORDER BY cached_at DESC, title ASC`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var notebooks []core.Notebook
	var latest *time.Time
	for rows.Next() {
		var n core.Notebook
		var isOwner int
		var cachedAt string
		if err := rows.Scan(&n.ID, &n.Title, &isOwner, &n.CreatedAt, &n.URL, &n.Emoji, &cachedAt); err != nil {
			return nil, nil, err
		}
		n.IsOwner = isOwner == 1
		if t, err := time.Parse(time.RFC3339, cachedAt); err == nil {
			n.CachedAt = t
			if latest == nil || t.After(*latest) {
				latest = &t
			}
		}
		notebooks = append(notebooks, n)
	}
	return notebooks, latest, rows.Err()
}

func (s *Store) ReplaceNotebooks(ctx context.Context, notebooks []core.Notebook) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM notebooks`); err != nil {
		return err
	}
	for _, n := range notebooks {
		if n.Emoji == "" {
			n.Emoji = "📒"
		}
		if n.CachedAt.IsZero() {
			n.CachedAt = time.Now().UTC()
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO notebooks(id, title, is_owner, created_at, url, emoji, cached_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
			n.ID, n.Title, boolToInt(n.IsOwner), n.CreatedAt, n.URL, n.Emoji, n.CachedAt.Format(time.RFC3339)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) InsertNotebook(ctx context.Context, notebook core.Notebook) error {
	if notebook.Emoji == "" {
		notebook.Emoji = "📒"
	}
	if notebook.CachedAt.IsZero() {
		notebook.CachedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO notebooks(id, title, is_owner, created_at, url, emoji, cached_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
title=excluded.title,
is_owner=excluded.is_owner,
created_at=excluded.created_at,
url=excluded.url,
emoji=excluded.emoji,
cached_at=excluded.cached_at`,
		notebook.ID, notebook.Title, boolToInt(notebook.IsOwner), notebook.CreatedAt, notebook.URL, notebook.Emoji, notebook.CachedAt.Format(time.RFC3339))
	return err
}

func (s *Store) NotebookTitle(ctx context.Context, notebookID string) (string, error) {
	var title string
	err := s.db.QueryRowContext(ctx, `SELECT title FROM notebooks WHERE id = ?`, notebookID).Scan(&title)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return title, err
}

func (s *Store) NotebookURL(ctx context.Context, notebookID string) (string, error) {
	var url string
	err := s.db.QueryRowContext(ctx, `SELECT url FROM notebooks WHERE id = ?`, notebookID).Scan(&url)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return url, err
}

func (s *Store) ListUploadedNotebooks(ctx context.Context, rawURL string) ([]core.UploadedNotebook, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT
	u.notebook_id,
	COALESCE(NULLIF(n.title, ''), u.notebook_id) AS title,
	COALESCE(n.is_owner, 1) AS is_owner,
	COALESCE(n.created_at, '') AS created_at,
	COALESCE(n.url, '') AS notebook_url,
	COALESCE(n.emoji, '📒') AS emoji,
	COALESCE(n.cached_at, '') AS cached_at,
	u.source_id,
	u.updated_at AS last_used
FROM uploaded_sources u
LEFT JOIN notebooks n ON n.id = u.notebook_id
WHERE u.url = ?
ORDER BY u.updated_at DESC`, rawURL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notebooks []core.UploadedNotebook
	byID := map[string]int{}
	sourceSeen := map[string]map[string]bool{}
	for rows.Next() {
		var n core.UploadedNotebook
		var isOwner int
		var cachedAt string
		var sourceID string
		if err := rows.Scan(
			&n.ID,
			&n.Title,
			&isOwner,
			&n.CreatedAt,
			&n.URL,
			&n.Emoji,
			&cachedAt,
			&sourceID,
			&n.LastUsed,
		); err != nil {
			return nil, err
		}
		n.IsOwner = isOwner == 1
		if t, err := time.Parse(time.RFC3339, cachedAt); err == nil {
			n.CachedAt = t
		}

		idx, ok := byID[n.ID]
		if !ok {
			n.SourceIDs = []string{}
			notebooks = append(notebooks, n)
			idx = len(notebooks) - 1
			byID[n.ID] = idx
			sourceSeen[n.ID] = map[string]bool{}
		}
		if sourceID != "" && !sourceSeen[n.ID][sourceID] {
			notebooks[idx].SourceIDs = append(notebooks[idx].SourceIDs, sourceID)
			sourceSeen[n.ID][sourceID] = true
		}
	}
	return notebooks, rows.Err()
}

func (s *Store) RecordUploadedSource(ctx context.Context, rawURL, notebookID, sourceID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
INSERT INTO uploaded_sources(url, notebook_id, source_id, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(url, notebook_id, source_id) DO UPDATE SET updated_at=excluded.updated_at`,
		rawURL, notebookID, sourceID, now, now)
	return err
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
