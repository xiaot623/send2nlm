package scriptmgr

import (
	"context"
	"fmt"
	"sync"

	"send2nlm/sdk"
)

// ProducerRegistry manages registered URL→PDF producers.
// Scripts (from ~/.send2nlm/producer/) take priority over builtins.
type ProducerRegistry struct {
	builtins []sdk.Producer // default fallback (compiled in)
	scripts  []sdk.Producer // from .go scripts (includes Lark)
	mu       sync.RWMutex
}

// NewProducerRegistry creates a new producer registry.
func NewProducerRegistry() *ProducerRegistry {
	return &ProducerRegistry{}
}

// RegisterBuiltin registers a compiled-in producer (e.g. Default).
func (r *ProducerRegistry) RegisterBuiltin(p sdk.Producer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.builtins = append(r.builtins, p)
}

// SetScripts replaces the list of script-loaded producers atomically.
func (r *ProducerRegistry) SetScripts(producers []sdk.Producer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scripts = producers
}

// Resolve finds the first producer that matches the URL and produces a PDF.
// Scripts are tried first (so Lark overrides Default), then builtins.
// Returns an error if no producer matches.
func (r *ProducerRegistry) Resolve(ctx context.Context, url string) (pdfPath string, err error) {
	r.mu.RLock()
	scripts := make([]sdk.Producer, len(r.scripts))
	copy(scripts, r.scripts)
	builtins := make([]sdk.Producer, len(r.builtins))
	copy(builtins, r.builtins)
	r.mu.RUnlock()

	// Scripts first (user extensions have higher priority)
	for _, p := range scripts {
		if p.Match(url) {
			return p.Produce(ctx, url)
		}
	}

	// Builtins as fallback
	for _, p := range builtins {
		if p.Match(url) {
			return p.Produce(ctx, url)
		}
	}

	return "", fmt.Errorf("no producer matched URL: %s", url)
}
