package server

import (
	"net/http"

	"send2nlm/core"
	"send2nlm/pipeline"
	"send2nlm/scriptmgr"
	"send2nlm/store"
)

type App struct {
	cfg      core.RuntimeConfig
	store    *store.Store
	version   string
	pipeline  core.JobExecutor
	producers *scriptmgr.ProducerRegistry
}

func NewApp(cfg core.RuntimeConfig, st *store.Store, version string, producers *scriptmgr.ProducerRegistry, receivers *scriptmgr.ReceiverRegistry) *App {
	return &App{
		cfg:       cfg,
		store:     st,
		version:   version,
		pipeline:  pipeline.New(cfg, st, producers, receivers),
		producers: producers,
	}
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", a.handleHealth)
	mux.HandleFunc("GET /notebooks", a.handleListNotebooks)
	mux.HandleFunc("POST /notebooks", a.handleCreateNotebook)
	mux.HandleFunc("POST /notebooks/{id}/upload", a.handleUploadResource)
	mux.HandleFunc("GET /notebooks/{id}/sources", a.handleListSources)
	mux.HandleFunc("GET /jobs", a.handleListJobs)
	mux.HandleFunc("POST /jobs", a.handleCreateJob)
	mux.HandleFunc("GET /jobs/", a.handleGetJob)
	return withMiddleware(mux)
}
