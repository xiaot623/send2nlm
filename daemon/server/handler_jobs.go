package server

import (
	"context"
	"net/http"
	"strings"

	"send2nlm/core"
)

func (a *App) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NotebookID string   `json:"notebook_id"`
		URL        string   `json:"url"` // Keep URL for metadata display
		Tasks      []string `json:"tasks"`
		SourceIDs  []string `json:"source_ids"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.NotebookID == "" || len(req.SourceIDs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "notebook_id and source_ids are required"})
		return
	}

	job := core.NewJob(req.NotebookID, notebookTitleLookup(r.Context(), a, req.NotebookID), req.URL, req.Tasks, req.SourceIDs)
	if err := a.store.CreateJob(job); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	_ = a.pipeline.Enqueue(context.Background(), job)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"job_id":      job.ID,
		"status":      "accepted",
		"notebook_id": job.NotebookID,
		"url":         job.URL,
	})
}

func (a *App) handleGetJob(w http.ResponseWriter, r *http.Request) {
	jobID := strings.TrimPrefix(r.URL.Path, "/jobs/")
	if jobID == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "job id is required"})
		return
	}
	job, err := a.store.GetJob(r.Context(), jobID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if job == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (a *App) handleListJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := a.store.ListJobs(r.Context(), r.URL.Query().Get("status"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

func (a *App) handleClearJobs(w http.ResponseWriter, r *http.Request) {
	if err := a.store.ClearCompletedOrFailedJobs(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
