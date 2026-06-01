package pipeline

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"send2nlm/core"
	"send2nlm/nlm"
	"send2nlm/scriptmgr"
	"send2nlm/sdk"
	"send2nlm/store"
)

// Pipeline orchestrates the full URL → PDF → NotebookLM flow.
type Pipeline struct {
	cfg       core.RuntimeConfig
	store     *store.Store
	producers *scriptmgr.ProducerRegistry
	receivers *scriptmgr.ReceiverRegistry
	queue     chan *core.Job
}

func New(cfg core.RuntimeConfig, st *store.Store, producers *scriptmgr.ProducerRegistry, receivers *scriptmgr.ReceiverRegistry) *Pipeline {
	p := &Pipeline{
		cfg:       cfg,
		store:     st,
		producers: producers,
		receivers: receivers,
		queue:     make(chan *core.Job, 16),
	}
	go p.loop()
	go p.resumeIncomplete()
	return p
}

func (p *Pipeline) Enqueue(_ context.Context, job *core.Job) error {
	p.queue <- job
	return nil
}

func (p *Pipeline) loop() {
	for job := range p.queue {
		func() {
			defer func() {
				if r := recover(); r != nil {
					println("pipeline panic:", fmt.Sprint(r))
				}
			}()
			_ = p.execute(job)
		}()
	}
}

func (p *Pipeline) resumeIncomplete() {
	jobs, err := p.store.ListResumableJobs(context.Background())
	if err != nil {
		log.Printf("[pipeline] resume scan failed: %v", err)
		return
	}
	for i := range jobs {
		job := jobs[i]
		log.Printf("[pipeline] resuming job %s from status %s", job.ID, job.Status)
		if err := p.Enqueue(context.Background(), &job); err != nil {
			log.Printf("[pipeline] resume enqueue failed for %s: %v", job.ID, err)
		}
	}
}

func (p *Pipeline) execute(job *core.Job) error {
	ctx := context.Background()
	fail := func(err error) error {
		_ = p.store.UpdateJobProgress(ctx, job.ID, map[string]any{
			"status": core.StatusFailed,
			"error":  err.Error(),
		})
		return err
	}

	switch job.Status {
	case core.StatusTasking:
		taskResults := job.TaskResults
		genTasks := genTasksFromResults(taskResults)
		if len(genTasks) > 0 {
			if job.PollingStartedAt == "" {
				job.PollingStartedAt = earliestTaskStartedAt(taskResults).Format(time.RFC3339)
			}
			_ = p.store.UpdateJobProgress(ctx, job.ID, map[string]any{
				"status":             core.StatusPolling,
				"polling_started_at": job.PollingStartedAt,
			})
			if err := p.pollUntilReady(ctx, job, genTasks); err != nil {
				_ = p.store.UpdateJobProgress(ctx, job.ID, map[string]any{"task_results": core.MarshalTaskResults(taskResultsFromGenTasks(taskResults, genTasks))})
				return fail(err)
			}
			return p.downloadAndReceive(ctx, job, taskResults, genTasks, fail)
		}
	case core.StatusPolling:
		taskResults := job.TaskResults
		genTasks := genTasksFromResults(taskResults)
		if len(genTasks) == 0 {
			return fail(fmt.Errorf("cannot resume polling without task results"))
		}
		if err := p.pollUntilReady(ctx, job, genTasks); err != nil {
			_ = p.store.UpdateJobProgress(ctx, job.ID, map[string]any{"task_results": core.MarshalTaskResults(taskResultsFromGenTasks(taskResults, genTasks))})
			return fail(err)
		}
		return p.downloadAndReceive(ctx, job, taskResults, genTasks, fail)
	case core.StatusDownloading:
		taskResults := job.TaskResults
		genTasks := genTasksFromResults(taskResults)
		if len(genTasks) == 0 {
			return fail(fmt.Errorf("cannot resume downloading without task results"))
		}
		return p.downloadAndReceive(ctx, job, taskResults, genTasks, fail)
	case core.StatusReceiving:
		return p.receive(ctx, job, job.TaskResults)
	}

	// ── Step 3: TASKING ─────────────────────────────────────────────────
	_ = p.store.UpdateJobStatus(ctx, job.ID, core.StatusTasking)
	taskResults := map[string]core.TaskResult{}
	genTasks := map[string]*nlm.GenTaskResponse{}

	for _, task := range job.Tasks {
		switch task {
		case "audio_overview":
			resp, err := nlm.GenerateAudio(ctx, job.NotebookID, job.SourceIDs)
			if err != nil {
				return fail(err)
			}
			genTasks[task] = resp
			taskResults[task] = core.TaskResult{
				TaskID:    resp.TaskID,
				Status:    resp.Status,
				StartedAt: time.Now().UTC().Format(time.RFC3339),
			}
		case "slide_deck":
			resp, err := nlm.GenerateSlides(ctx, job.NotebookID, job.SourceIDs)
			if err != nil {
				return fail(err)
			}
			genTasks[task] = resp
			taskResults[task] = core.TaskResult{
				TaskID:    resp.TaskID,
				Status:    resp.Status,
				StartedAt: time.Now().UTC().Format(time.RFC3339),
			}
		case "video_overview":
			resp, err := nlm.GenerateVideo(ctx, job.NotebookID, job.SourceIDs)
			if err != nil {
				return fail(err)
			}
			genTasks[task] = resp
			taskResults[task] = core.TaskResult{
				TaskID:    resp.TaskID,
				Status:    resp.Status,
				StartedAt: time.Now().UTC().Format(time.RFC3339),
			}
		}
	}
	job.TaskResults = taskResults
	_ = p.store.UpdateJobProgress(ctx, job.ID, map[string]any{"task_results": core.MarshalTaskResults(taskResults)})

	// No tasks requested — finish early.
	if len(genTasks) == 0 {
		_ = p.store.UpdateJobProgress(ctx, job.ID, map[string]any{
			"status":       core.StatusDone,
			"completed_at": time.Now().UTC().Format(time.RFC3339),
		})
		return nil
	}

	// ── Step 4: POLLING ─────────────────────────────────────────────────
	if job.PollingStartedAt == "" {
		job.PollingStartedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_ = p.store.UpdateJobProgress(ctx, job.ID, map[string]any{
		"status":             core.StatusPolling,
		"polling_started_at": job.PollingStartedAt,
	})
	if err := p.pollUntilReady(ctx, job, genTasks); err != nil {
		_ = p.store.UpdateJobProgress(ctx, job.ID, map[string]any{"task_results": core.MarshalTaskResults(taskResultsFromGenTasks(taskResults, genTasks))})
		return fail(err)
	}
	return p.downloadAndReceive(ctx, job, taskResults, genTasks, fail)
}

func (p *Pipeline) pollUntilReady(ctx context.Context, job *core.Job, genTasks map[string]*nlm.GenTaskResponse) error {
	const (
		initialWait = 10 * time.Minute
		totalLimit  = 1 * time.Hour
		interval    = 1 * time.Minute
	)

	startedAt := time.Now().UTC()
	if job.PollingStartedAt != "" {
		if parsed, err := time.Parse(time.RFC3339, job.PollingStartedAt); err == nil {
			startedAt = parsed
		}
	}
	if job.PollingStartedAt == "" {
		job.PollingStartedAt = startedAt.Format(time.RFC3339)
		_ = p.store.UpdateJobProgress(ctx, job.ID, map[string]any{
			"status":             core.StatusPolling,
			"polling_started_at": job.PollingStartedAt,
		})
	}

	elapsed := time.Since(startedAt)
	if elapsed >= totalLimit {
		allReady, err := nlm.PollTasksOnce(ctx, job.NotebookID, genTasks)
		if err != nil {
			return err
		}
		if allReady {
			return nil
		}
		return fmt.Errorf("timed out waiting for artifact completion after %v", totalLimit)
	}

	delay := time.Duration(0)
	if elapsed < initialWait {
		delay = initialWait - elapsed
	}
	return nlm.PollUntilReady(ctx, job.NotebookID, genTasks, delay, totalLimit-elapsed, interval)
}

func (p *Pipeline) downloadAndReceive(ctx context.Context, job *core.Job, taskResults map[string]core.TaskResult, genTasks map[string]*nlm.GenTaskResponse, fail func(error) error) error {
	// Update task results with final statuses after polling completes.
	for taskType, genTask := range genTasks {
		tr := taskResults[taskType]
		tr.Status = genTask.Status
		tr.CompletedAt = time.Now().UTC().Format(time.RFC3339)
		taskResults[taskType] = tr
	}
	_ = p.store.UpdateJobProgress(ctx, job.ID, map[string]any{"task_results": core.MarshalTaskResults(taskResults)})

	// ── Step 5: DOWNLOADING ─────────────────────────────────────────────
	_ = p.store.UpdateJobStatus(ctx, job.ID, core.StatusDownloading)

	outputDir := filepathInTemp(p.cfg.TempDir(), job.ID)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fail(err)
	}

	downloads, err := nlm.DownloadArtifacts(ctx, job.NotebookID, outputDir, genTasks)
	if err != nil {
		return fail(err)
	}
	for _, d := range downloads {
		tr := taskResults[d.TaskType]
		tr.AssetPath = d.Path
		tr.Status = core.StatusDone
		taskResults[d.TaskType] = tr
	}
	_ = p.store.UpdateJobProgress(ctx, job.ID, map[string]any{"task_results": core.MarshalTaskResults(taskResults)})

	return p.receive(ctx, job, taskResults)
}

func (p *Pipeline) receive(ctx context.Context, job *core.Job, taskResults map[string]core.TaskResult) error {
	// ── Step 6: RECEIVING ───────────────────────────────────────────────
	_ = p.store.UpdateJobStatus(ctx, job.ID, core.StatusReceiving)
	resources := buildResources(job, taskResults)
	errs := p.receivers.Deliver(ctx, resources)
	for _, err := range errs {
		log.Printf("[pipeline] receiver delivery warning: %v", err)
	}

	// ── Step 7: DONE ────────────────────────────────────────────────────
	_ = p.store.UpdateJobProgress(ctx, job.ID, map[string]any{
		"status":       core.StatusDone,
		"task_results": core.MarshalTaskResults(taskResults),
		"completed_at": time.Now().UTC().Format(time.RFC3339),
	})
	return nil
}

func genTasksFromResults(results map[string]core.TaskResult) map[string]*nlm.GenTaskResponse {
	genTasks := make(map[string]*nlm.GenTaskResponse, len(results))
	for taskType, result := range results {
		if result.TaskID == "" {
			continue
		}
		genTasks[taskType] = &nlm.GenTaskResponse{
			TaskID:   result.TaskID,
			Status:   result.Status,
			TaskType: taskType,
		}
	}
	return genTasks
}

func taskResultsFromGenTasks(results map[string]core.TaskResult, genTasks map[string]*nlm.GenTaskResponse) map[string]core.TaskResult {
	for taskType, genTask := range genTasks {
		tr := results[taskType]
		tr.TaskID = genTask.TaskID
		tr.Status = genTask.Status
		results[taskType] = tr
	}
	return results
}

func earliestTaskStartedAt(results map[string]core.TaskResult) time.Time {
	startedAt := time.Now().UTC()
	found := false
	for _, result := range results {
		if result.StartedAt == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, result.StartedAt)
		if err != nil {
			continue
		}
		if !found || parsed.Before(startedAt) {
			startedAt = parsed
			found = true
		}
	}
	return startedAt.UTC()
}

// filepathInTemp builds a path under the temp directory.
func filepathInTemp(tmpDir, jobID string) string {
	return tmpDir + "/" + jobID
}

// buildResources converts core.TaskResult map to []sdk.Resource for receiver delivery.
func buildResources(job *core.Job, results map[string]core.TaskResult) []sdk.Resource {
	var out []sdk.Resource
	for taskType, tr := range results {
		mime := "application/octet-stream"
		switch taskType {
		case "audio_overview":
			mime = "audio/wav"
		case "slide_deck":
			mime = "application/pdf"
		case "video_overview":
			mime = "video/mp4"
		}
		r := sdk.Resource{
			TaskType:      taskType,
			AssetPath:     tr.AssetPath,
			MimeType:      mime,
			NotebookTitle: job.NotebookTitle,
			SourceURL:     job.URL,
		}
		r.DeliveryName = sdk.BuildDeliveryName(r)
		out = append(out, r)
	}
	return out
}
