package core

import "context"

type JobExecutor interface {
	Enqueue(ctx context.Context, job *Job) error
}

type NoopPipeline struct{}

func (p *NoopPipeline) Enqueue(_ context.Context, _ *Job) error {
	return nil
}
