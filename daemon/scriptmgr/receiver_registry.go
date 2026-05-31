package scriptmgr

import (
	"context"
	"log"
	"sync"

	"send2nlm/sdk"
)

// ReceiverRegistry manages registered artifact delivery adapters.
// All enabled receivers are invoked in registration order.
type ReceiverRegistry struct {
	builtins []sdk.Receiver // DownloadReceiver (compiled in)
	scripts  []sdk.Receiver // from .go scripts (includes Telegram)
	mu       sync.RWMutex
}

// NewReceiverRegistry creates a new receiver registry.
func NewReceiverRegistry() *ReceiverRegistry {
	return &ReceiverRegistry{}
}

// RegisterBuiltin registers a compiled-in receiver (e.g. Download).
func (r *ReceiverRegistry) RegisterBuiltin(rec sdk.Receiver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.builtins = append(r.builtins, rec)
}

// SetScripts replaces the list of script-loaded receivers atomically.
func (r *ReceiverRegistry) SetScripts(receivers []sdk.Receiver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scripts = receivers
}

// Deliver invokes all registered receivers in order (scripts first, then builtins).
// Each receiver can decide to skip itself by checking config.
// Individual receiver failures are logged but do not stop the chain.
func (r *ReceiverRegistry) Deliver(ctx context.Context, resources []sdk.Resource) []error {
	r.mu.RLock()
	scripts := make([]sdk.Receiver, len(r.scripts))
	copy(scripts, r.scripts)
	builtins := make([]sdk.Receiver, len(r.builtins))
	copy(builtins, r.builtins)
	r.mu.RUnlock()

	var errs []error

	all := append(scripts, builtins...)
	for _, rec := range all {
		cfg := sdk.LoadConfig()
		if rc, ok := cfg.Receivers[rec.Name()]; ok {
			if enabled, ok := rc["enabled"].(bool); ok && !enabled {
				continue
			}
		}
		if err := rec.Receive(ctx, resources); err != nil {
			log.Printf("[receiver] %s delivery failed: %v", rec.Name(), err)
			errs = append(errs, err)
		}
	}

	return errs
}
