// Convert Audio to MP3 Aspect — converts downloaded audio (e.g., w4a/m4a/wav) to mp3 using ffmpeg.
// Copy to ~/.send2nlm/aspect/receive/ or dev_assets/aspect/receive/ to enable.

package aspect

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"send2nlm/sdk"
)

type ConvertToMP3Aspect struct{}

func (a *ConvertToMP3Aspect) Name() string { return "convert-to-mp3" }

func (a *ConvertToMP3Aspect) Priority() int { return 10 }

func (a *ConvertToMP3Aspect) BeforeReceive(ctx context.Context, resources []sdk.Resource) ([]sdk.Resource, error) {
	var updatedResources []sdk.Resource

	for _, res := range resources {
		// Only process audio_overview or files that look like audio
		if res.TaskType != "audio_overview" && !strings.HasPrefix(res.MimeType, "audio/") {
			updatedResources = append(updatedResources, res)
			continue
		}

		ext := strings.ToLower(filepath.Ext(res.AssetPath))
		if ext == ".mp3" {
			// Already mp3
			updatedResources = append(updatedResources, res)
			continue
		}

		log.Printf("[aspect:%s] Converting %s to mp3...", a.Name(), res.AssetPath)

		// Create output path
		outDir := filepath.Dir(res.AssetPath)
		baseName := strings.TrimSuffix(filepath.Base(res.AssetPath), filepath.Ext(res.AssetPath))
		outPath := filepath.Join(outDir, baseName+".mp3")

		// Call ffmpeg to convert and compress
		// -b:a 64k is a good balance for speech/podcast compression
		cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", res.AssetPath, "-codec:a", "libmp3lame", "-b:a", "64k", outPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			log.Printf("[aspect:%s] ffmpeg conversion failed: %v\nOutput: %s", a.Name(), err, string(output))
			// Fallback to original resource if conversion fails
			updatedResources = append(updatedResources, res)
			continue
		}

		log.Printf("[aspect:%s] Successfully converted to %s", a.Name(), outPath)

		// Clean up the original file if desired
		os.Remove(res.AssetPath)

		// Update resource properties
		res.AssetPath = outPath
		res.MimeType = "audio/mpeg"
		
		// Update DeliveryName extension
		if res.DeliveryName != "" {
			oldExt := filepath.Ext(res.DeliveryName)
			res.DeliveryName = strings.TrimSuffix(res.DeliveryName, oldExt) + ".mp3"
		}

		updatedResources = append(updatedResources, res)
	}

	return updatedResources, nil
}

var ReceiveAspect sdk.ReceiveAspect = &ConvertToMP3Aspect{}
