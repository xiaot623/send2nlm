package server

import (
	"context"
	"net/http"
	"time"

	"send2nlm/core"
	"send2nlm/nlm"
)

func (a *App) handleListNotebooks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	refresh := r.URL.Query().Get("refresh") == "true"
	fromCache := true

	notebooks, cachedAt, err := a.store.ListNotebooks(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if refresh || len(notebooks) == 0 {
		fromCache = false
		fresh, err := nlm.ListNotebooks(ctx)
		if err != nil {
			status := http.StatusBadGateway
			if len(notebooks) > 0 {
				writeJSON(w, http.StatusOK, map[string]any{
					"notebooks":  notebooks,
					"from_cache": true,
					"cached_at":  cachedAtString(cachedAt),
					"warning":    err.Error(),
				})
				return
			}
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		if err := a.store.ReplaceNotebooks(ctx, fresh); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		notebooks = fresh
		now := time.Now().UTC()
		cachedAt = &now
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"notebooks":  notebooks,
		"from_cache": fromCache,
		"cached_at":  cachedAtString(cachedAt),
	})
}

func (a *App) handleCreateNotebook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string `json:"title"`
		Emoji string `json:"emoji"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
		return
	}

	notebook, err := nlm.CreateNotebook(r.Context(), req.Title, req.Emoji)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if err := a.store.InsertNotebook(r.Context(), *notebook); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, notebook)
}

func cachedAtString(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

func notebookTitleLookup(ctx context.Context, a *App, notebookID string) string {
	title, err := a.store.NotebookTitle(ctx, notebookID)
	if err != nil {
		return ""
	}
	return title
}

var _ = core.Notebook{}
