package server

import (
	"net/http"

	"send2nlm/core"
	"send2nlm/pipeline"
	"send2nlm/store"
)

type App struct {
	cfg      core.RuntimeConfig
	store    *store.Store
	version  string
	pipeline core.JobExecutor
}

func NewApp(cfg core.RuntimeConfig, st *store.Store, version string) *App {
	return &App{
		cfg:      cfg,
		store:    st,
		version:  version,
		pipeline: pipeline.New(cfg, st),
	}
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", a.handleHealth)
	mux.HandleFunc("GET /notebooks", a.handleListNotebooks)
	mux.HandleFunc("POST /notebooks", a.handleCreateNotebook)
	mux.HandleFunc("GET /jobs", a.handleListJobs)
	mux.HandleFunc("POST /jobs", a.handleCreateJob)
	mux.HandleFunc("GET /jobs/", a.handleGetJob)
	return withMiddleware(mux)
}
