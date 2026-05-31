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

func (p *Pipeline) execute(job *core.Job) error {
	ctx := context.Background()
	fail := func(err error) error {
		_ = p.store.UpdateJobProgress(ctx, job.ID, map[string]any{
			"status": core.StatusFailed,
			"error":  err.Error(),
		})
		return err
	}

	// ── Step 1: PRODUCING ───────────────────────────────────────────────
	_ = p.store.UpdateJobStatus(ctx, job.ID, core.StatusProducing)
	pdfPath, err := p.producers.Resolve(ctx, job.URL)
	if err != nil {
		return fail(err)
	}
	job.PDFPath = pdfPath
	_ = p.store.UpdateJobProgress(ctx, job.ID, map[string]any{"pdf_path": pdfPath})

	// ── Step 2: UPLOADING ───────────────────────────────────────────────
	_ = p.store.UpdateJobStatus(ctx, job.ID, core.StatusUploading)
	sourceID, err := nlm.AddFileSource(ctx, job.NotebookID, pdfPath)
	if err != nil {
		return fail(err)
	}
	job.SourceID = sourceID
	_ = p.store.UpdateJobProgress(ctx, job.ID, map[string]any{"source_id": sourceID})

	// ── Step 3: TASKING ─────────────────────────────────────────────────
	_ = p.store.UpdateJobStatus(ctx, job.ID, core.StatusTasking)
	taskResults := map[string]core.TaskResult{}
	genTasks := map[string]*nlm.GenTaskResponse{}

	for _, task := range job.Tasks {
		switch task {
		case "audio_overview":
			resp, err := nlm.GenerateAudio(ctx, job.NotebookID)
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
			resp, err := nlm.GenerateSlides(ctx, job.NotebookID)
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
	_ = p.store.UpdateJobStatus(ctx, job.ID, core.StatusPolling)
	if err := nlm.PollUntilReady(ctx, job.NotebookID, genTasks, 40*time.Minute, 30*time.Second); err != nil {
		return fail(err)
	}
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
		}
		out = append(out, sdk.Resource{
			TaskType:      taskType,
			AssetPath:     tr.AssetPath,
			MimeType:      mime,
			NotebookTitle: job.NotebookTitle,
			SourceURL:     job.URL,
		})
	}
	return out
}
