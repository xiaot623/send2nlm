package store

import (
	"context"
	"database/sql"
	"time"

	"send2nlm/core"
)

func (s *Store) ListNotebooks(ctx context.Context) ([]core.Notebook, *time.Time, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, title, is_owner, created_at, url, source_count, emoji, cached_at
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
		if err := rows.Scan(&n.ID, &n.Title, &isOwner, &n.CreatedAt, &n.URL, &n.SourceCount, &n.Emoji, &cachedAt); err != nil {
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
INSERT INTO notebooks(id, title, is_owner, created_at, url, source_count, emoji, cached_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			n.ID, n.Title, boolToInt(n.IsOwner), n.CreatedAt, n.URL, n.SourceCount, n.Emoji, n.CachedAt.Format(time.RFC3339)); err != nil {
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
INSERT INTO notebooks(id, title, is_owner, created_at, url, source_count, emoji, cached_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
title=excluded.title,
is_owner=excluded.is_owner,
created_at=excluded.created_at,
url=excluded.url,
source_count=excluded.source_count,
emoji=excluded.emoji,
cached_at=excluded.cached_at`,
		notebook.ID, notebook.Title, boolToInt(notebook.IsOwner), notebook.CreatedAt, notebook.URL, notebook.SourceCount, notebook.Emoji, notebook.CachedAt.Format(time.RFC3339))
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

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
