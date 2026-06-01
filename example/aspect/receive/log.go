// Log Receive Aspect — records delivery resources and returns them unchanged.
// Copy to ~/.send2nlm/aspect/receive/ or dev_assets/aspect/receive/ to enable.

package aspect

import (
	"context"
	"log"

	"send2nlm/sdk"
)

type LogReceiveAspect struct{}

func (a *LogReceiveAspect) Name() string { return "log-receive" }

func (a *LogReceiveAspect) Priority() int { return 0 }

func (a *LogReceiveAspect) BeforeReceive(ctx context.Context, resources []sdk.Resource) ([]sdk.Resource, error) {
	log.Printf("[aspect:%s] resources=%d", a.Name(), len(resources))
	for _, res := range resources {
		log.Printf("[aspect:%s] task=%s asset=%s delivery=%s", a.Name(), res.TaskType, res.AssetPath, res.DeliveryName)
	}
	return resources, nil
}

var ReceiveAspect sdk.ReceiveAspect = &LogReceiveAspect{}
