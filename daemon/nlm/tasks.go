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
	TaskID string `json:"task_id"`
	Status string `json:"status"`
	Ready  bool   `json:"ready,omitempty"`
}

// DownloadedArtifact records a successfully downloaded artifact.
type DownloadedArtifact struct {
	TaskType string
	TaskID   string
	Path     string
}

// GenerateAudio starts an Audio Overview generation via `notebooklm generate audio`.
func GenerateAudio(ctx context.Context, notebookID string, sourceIDs []string) (*GenTaskResponse, error) {
	args := []string{"generate", "audio", "-n", notebookID}
	for _, sid := range sourceIDs {
		args = append(args, "-s", sid)
	}
	args = append(args, "Create a deep-dive audio overview summarizing the content", "--json")
	out, err := execNotebookLM(ctx, args...)
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
func GenerateSlides(ctx context.Context, notebookID string, sourceIDs []string) (*GenTaskResponse, error) {
	args := []string{"generate", "slide-deck", "-n", notebookID}
	for _, sid := range sourceIDs {
		args = append(args, "-s", sid)
	}
	args = append(args, "--json")
	out, err := execNotebookLM(ctx, args...)
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

// GenerateVideo starts a Video Overview generation via `notebooklm generate video`.
func GenerateVideo(ctx context.Context, notebookID string, sourceIDs []string) (*GenTaskResponse, error) {
	args := []string{"generate", "video", "-n", notebookID}
	for _, sid := range sourceIDs {
		args = append(args, "-s", sid)
	}
	args = append(args, "Create a concise video overview summarizing the content", "--json")
	out, err := execNotebookLM(ctx, args...)
	if err != nil {
		return nil, err
	}
	var resp GenTaskResponse
	if err := decodeJSON(out, &resp); err != nil {
		return nil, fmt.Errorf("decode generate video: %w", err)
	}
	resp.TaskType = "video_overview"
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

func PollUntilReady(ctx context.Context, notebookID string, tasks map[string]*GenTaskResponse, initialDelay, timeout, interval time.Duration) error {
	if usingMockBackend() {
		allReady, err := PollTasksOnce(ctx, notebookID, tasks)
		if err != nil {
			return err
		}
		if allReady {
			return nil
		}
		return fmt.Errorf("mock NotebookLM tasks were not ready")
	}

	deadline := time.Now().Add(timeout)
	if initialDelay > 0 {
		timer := time.NewTimer(initialDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	if !time.Now().Before(deadline) {
		return pollOnceBeforeTimeout(ctx, notebookID, tasks, timeout)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	consecutiveErrors := 0
	const maxErrors = 3

	for time.Now().Before(deadline) {
		allReady, loopErr := PollTasksOnce(ctx, notebookID, tasks)
		if loopErr != nil {
			consecutiveErrors++
			if consecutiveErrors >= maxErrors {
				return fmt.Errorf("failed to poll after %d attempts: %w", maxErrors, loopErr)
			}
		} else {
			consecutiveErrors = 0
			if allReady {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
	return pollOnceBeforeTimeout(ctx, notebookID, tasks, timeout)
}

func pollOnceBeforeTimeout(ctx context.Context, notebookID string, tasks map[string]*GenTaskResponse, timeout time.Duration) error {
	allReady, err := PollTasksOnce(ctx, notebookID, tasks)
	if err != nil {
		return err
	}
	if allReady {
		return nil
	}
	return fmt.Errorf("timed out waiting for artifact completion after %v", timeout)
}

func PollTasksOnce(ctx context.Context, notebookID string, tasks map[string]*GenTaskResponse) (bool, error) {
	allReady := true
	for taskType, task := range tasks {
		state, err := PollArtifact(ctx, notebookID, task.TaskID)
		if err != nil {
			return false, fmt.Errorf("poll %s: %w", taskType, err)
		}
		// Update the task status for the caller.
		task.Status = state.Status
		switch state.Status {
		case "completed", "done", "ready":
			// Task finished successfully.
		case "failed", "error":
			return false, fmt.Errorf("task %s (%s) failed: status=%s", taskType, task.TaskID, state.Status)
		default:
			// Still processing — "pending", "generating", etc.
			allReady = false
		}
	}
	return allReady, nil
}

// DownloadArtifacts downloads all completed artifacts to the given directory
// via `notebooklm download <type> <path> -a <artifactID> --force`.
// Uses the exact artifact ID from generate/poll to avoid ambiguity.
func DownloadArtifacts(ctx context.Context, notebookID, outputDir string, tasks map[string]*GenTaskResponse) ([]DownloadedArtifact, error) {
	results := make([]DownloadedArtifact, 0, len(tasks))

	for taskType, task := range tasks {
		var outputPath string
		var downloadArgs []string

		switch taskType {
		case "audio_overview":
			outputPath = filepath.Join(outputDir, fmt.Sprintf("audio_%s.wav", shortTaskID(task.TaskID)))
			downloadArgs = []string{
				"download", "audio",
				"-n", notebookID,
				"-a", task.TaskID,
				outputPath,
				"--force",
			}
		case "slide_deck":
			outputPath = filepath.Join(outputDir, fmt.Sprintf("slides_%s.pdf", shortTaskID(task.TaskID)))
			downloadArgs = []string{
				"download", "slide-deck",
				"-n", notebookID,
				"-a", task.TaskID,
				outputPath,
				"--force",
			}
		case "video_overview":
			outputPath = filepath.Join(outputDir, fmt.Sprintf("video_%s.mp4", shortTaskID(task.TaskID)))
			downloadArgs = []string{
				"download", "video",
				"-n", notebookID,
				"-a", task.TaskID,
				outputPath,
				"--force",
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

func shortTaskID(taskID string) string {
	if len(taskID) <= 8 {
		return taskID
	}
	return taskID[:8]
}
