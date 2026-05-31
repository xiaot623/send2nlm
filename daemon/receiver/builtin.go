package receiver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"send2nlm/sdk"
)

// DownloadReceiver copies pipeline artifacts to ~/Downloads/send2nlm/.
// This is the only receiver compiled into the daemon binary.
type DownloadReceiver struct {
	outputDir string
}

// NewDownloadReceiver creates a receiver that copies files to the Downloads folder.
func NewDownloadReceiver() *DownloadReceiver {
	home, _ := os.UserHomeDir()
	return &DownloadReceiver{
		outputDir: filepath.Join(home, "Downloads", "send2nlm"),
	}
}

func (r *DownloadReceiver) Name() string { return "download" }

func (r *DownloadReceiver) Receive(ctx context.Context, resources []sdk.Resource) error {
	if err := os.MkdirAll(r.outputDir, 0o755); err != nil {
		return fmt.Errorf("download receiver: %w", err)
	}

	for _, res := range resources {
		if res.AssetPath == "" {
			continue
		}
		src, err := os.ReadFile(res.AssetPath)
		if err != nil {
			return fmt.Errorf("download receiver: read %s: %w", res.AssetPath, err)
		}
		dst := filepath.Join(r.outputDir, res.DeliveryName)
		if err := os.WriteFile(dst, src, 0o644); err != nil {
			return fmt.Errorf("download receiver: write %s: %w", dst, err)
		}
	}
	return nil
}
