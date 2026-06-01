package sdk

import "context"

// URLAspect transforms an incoming URL before producer resolution.
// Each URL aspect script must export a package-level variable named "URLAspect"
// whose type implements this interface.
type URLAspect interface {
	Name() string
	Priority() int
	OnURL(ctx context.Context, url string) (string, error)
}

// ReceiveAspect transforms delivery resources before receiver execution.
// Each receive aspect script must export a package-level variable named
// "ReceiveAspect" whose type implements this interface.
type ReceiveAspect interface {
	Name() string
	Priority() int
	BeforeReceive(ctx context.Context, resources []Resource) ([]Resource, error)
}
