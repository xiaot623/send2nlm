package scriptmgr

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"send2nlm/sdk"
)

// URLAspectRegistry manages URL aspect scripts.
type URLAspectRegistry struct {
	scripts []sdk.URLAspect
	mu      sync.RWMutex
}

func NewURLAspectRegistry() *URLAspectRegistry {
	return &URLAspectRegistry{}
}

func (r *URLAspectRegistry) SetScripts(aspects []sdk.URLAspect) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scripts = sortedURLAspects(aspects)
}

func (r *URLAspectRegistry) Apply(ctx context.Context, url string) (string, error) {
	r.mu.RLock()
	aspects := make([]sdk.URLAspect, len(r.scripts))
	copy(aspects, r.scripts)
	r.mu.RUnlock()

	current := url
	for _, aspect := range aspects {
		next, err := aspect.OnURL(ctx, current)
		if err != nil {
			return "", fmt.Errorf("url aspect %s: %w", aspect.Name(), err)
		}
		current = next
	}
	return current, nil
}

// ReceiveAspectRegistry manages receive aspect scripts.
type ReceiveAspectRegistry struct {
	scripts []sdk.ReceiveAspect
	mu      sync.RWMutex
}

func NewReceiveAspectRegistry() *ReceiveAspectRegistry {
	return &ReceiveAspectRegistry{}
}

func (r *ReceiveAspectRegistry) SetScripts(aspects []sdk.ReceiveAspect) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scripts = sortedReceiveAspects(aspects)
}

func (r *ReceiveAspectRegistry) Apply(ctx context.Context, resources []sdk.Resource) ([]sdk.Resource, error) {
	r.mu.RLock()
	aspects := make([]sdk.ReceiveAspect, len(r.scripts))
	copy(aspects, r.scripts)
	r.mu.RUnlock()

	current := resources
	for _, aspect := range aspects {
		next, err := aspect.BeforeReceive(ctx, current)
		if err != nil {
			return nil, fmt.Errorf("receive aspect %s: %w", aspect.Name(), err)
		}
		current = next
	}
	return current, nil
}

func sortedURLAspects(aspects []sdk.URLAspect) []sdk.URLAspect {
	out := append([]sdk.URLAspect(nil), aspects...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority() == out[j].Priority() {
			return out[i].Name() < out[j].Name()
		}
		return out[i].Priority() > out[j].Priority()
	})
	return out
}

func sortedReceiveAspects(aspects []sdk.ReceiveAspect) []sdk.ReceiveAspect {
	out := append([]sdk.ReceiveAspect(nil), aspects...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority() == out[j].Priority() {
			return out[i].Name() < out[j].Name()
		}
		return out[i].Priority() > out[j].Priority()
	})
	return out
}
