package sdk

import "context"

// Producer is the URL → PDF adapter interface.
// Each script file must export a package-level variable named "Producer"
// whose type implements this interface.
type Producer interface {
	Name() string
	Match(url string) bool
	Produce(ctx context.Context, url string) (pdfPath string, err error)
}
