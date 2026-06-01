// Log URL Aspect — records incoming URLs and returns them unchanged.
// Copy to ~/.send2nlm/aspect/url/ or dev_assets/aspect/url/ to enable.

package aspect

import (
	"context"
	"log"

	"send2nlm/sdk"
)

type LogURLAspect struct{}

func (a *LogURLAspect) Name() string { return "log-url" }

func (a *LogURLAspect) Priority() int { return 0 }

func (a *LogURLAspect) OnURL(ctx context.Context, url string) (string, error) {
	log.Printf("[aspect:%s] url=%s", a.Name(), url)
	return url, nil
}

var URLAspect sdk.URLAspect = &LogURLAspect{}
