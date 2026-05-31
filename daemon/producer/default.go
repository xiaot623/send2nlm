package producer

import (
	"context"
	"fmt"

	"send2nlm/sdk"
)

// DefaultProducer is the universal fallback producer.
// It matches any URL and handles HTTP fetch → MD → PDF conversion.
// This is the only producer compiled into the daemon binary.
type DefaultProducer struct{}

// Ensure DefaultProducer implements sdk.Producer.
var _ sdk.Producer = (*DefaultProducer)(nil)

func (p *DefaultProducer) Name() string { return "default" }

func (p *DefaultProducer) Match(url string) bool { return true }

func (p *DefaultProducer) Produce(ctx context.Context, url string) (string, error) {
	return "", fmt.Errorf("default producer: HTTP→MD→PDF conversion not yet implemented (see DESIGN.md §7.5)")
}
