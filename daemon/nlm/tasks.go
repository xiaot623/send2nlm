package nlm

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
)

// GenTaskResponse represents the JSON output from `notebooklm generate <type> --json`.
type GenTaskResponse struct {
	TaskID   string `json:"task_id"`
	Status   string `json:"status"`
	TaskType string `json:"task_type"` // set by caller
}

// PollResponse represents the JSON output from `notebooklm artifact poll <taskID> --json`.
type PollResponse struct {
	TaskID  string `json:"task_id"`
	Status  string `json:"status"`
	Ready   bool   `json:"ready,omitempty"`
}

// DownloadedArtifact records a successfully downloaded artifact.
type DownloadedArtifact struct {
	TaskType string
	TaskID   string
	Path     string
}

// GenerateAudio starts an Audio Overview generation via `notebooklm generate audio`.
func GenerateAudio(ctx context.Context, notebookID string) (*GenTaskResponse, error) {
	out, err := execNotebookLM(ctx,
		"generate", "audio",
		"-n", notebookID,
		"Create a deep-dive audio overview summarizing the content",
		"--json",
	)
	if err != nil {
		return nil, err
	}
	var resp GenTaskResponse
	if err := decodeJSON(out, &resp); err != nil {
		return nil, fmt.Errorf("decode generate audio: %w", err)
	}
	resp.TaskType = "audio_overview"
	return &resp, nil
}

// GenerateSlides starts a Slide Deck generation via `notebooklm generate slide-deck`.
func GenerateSlides(ctx context.Context, notebookID string) (*GenTaskResponse, error) {
	out, err := execNotebookLM(ctx,
		"generate", "slide-deck",
		"-n", notebookID,
		"--json",
	)
	if err != nil {
		return nil, err
	}
	var resp GenTaskResponse
	if err := decodeJSON(out, &resp); err != nil {
		return nil, fmt.Errorf("decode generate slides: %w", err)
	}
	resp.TaskType = "slide_deck"
	return &resp, nil
}

// PollArtifact checks the current status of a single artifact via `notebooklm artifact poll`.
func PollArtifact(ctx context.Context, notebookID, taskID string) (*PollResponse, error) {
	out, err := execNotebookLM(ctx,
		"artifact", "poll",
		"-n", notebookID,
		taskID,
		"--json",
	)
	if err != nil {
		return nil, err
	}
	var resp PollResponse
	if err := decodeJSON(out, &resp); err != nil {
		return nil, fmt.Errorf("decode artifact poll: %w", err)
	}
	return &resp, nil
}

// PollUntilReady blocks until all tasks are completed, failed, or the deadline is exceeded.
// It polls each task at the given interval.
func PollUntilReady(ctx context.Context, notebookID string, tasks map[string]*GenTaskResponse, timeout, interval time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		allReady := true
		for taskType, task := range tasks {
			state, err := PollArtifact(ctx, notebookID, task.TaskID)
			if err != nil {
				return fmt.Errorf("poll %s: %w", taskType, err)
			}
			// Update the task status for the caller.
			task.Status = state.Status
			switch state.Status {
			case "completed", "done", "ready":
				// Task finished successfully.
			case "failed", "error":
				return fmt.Errorf("task %s (%s) failed: status=%s", taskType, task.TaskID, state.Status)
			default:
				// Still processing — "pending", "generating", etc.
				allReady = false
			}
		}
		if allReady {
			return nil
		}
		<-ticker.C
	}
	return fmt.Errorf("timed out waiting for artifact completion after %v", timeout)
}

// DownloadArtifacts downloads all completed artifacts to the given directory
// via `notebooklm download <type> <path> --latest --force`.
func DownloadArtifacts(ctx context.Context, notebookID, outputDir string, tasks map[string]*GenTaskResponse) ([]DownloadedArtifact, error) {
	results := make([]DownloadedArtifact, 0, len(tasks))

	for taskType, task := range tasks {
		var outputPath string
		var downloadArgs []string

		switch taskType {
		case "audio_overview":
			outputPath = filepath.Join(outputDir, fmt.Sprintf("audio_%s.wav", task.TaskID[:8]))
			downloadArgs = []string{
				"download", "audio",
				"-n", notebookID,
				outputPath,
				"--latest", "--force",
			}
		case "slide_deck":
			outputPath = filepath.Join(outputDir, fmt.Sprintf("slides_%s.pdf", task.TaskID[:8]))
			downloadArgs = []string{
				"download", "slide-deck",
				"-n", notebookID,
				outputPath,
				"--latest", "--force",
			}
		default:
			continue
		}

		if _, err := execNotebookLM(ctx, downloadArgs...); err != nil {
			return nil, fmt.Errorf("download %s: %w", taskType, err)
		}

		results = append(results, DownloadedArtifact{
			TaskType: taskType,
			TaskID:   task.TaskID,
			Path:     outputPath,
		})
	}
	return results, nil
}
