package jobs

import "context"

type IEntQueue interface {
	Enqueue(job Job) error
	Dequeue(ctx context.Context) (Job, error)
	MarkDone(ctx context.Context, jobID string) error
	MarkFailed(ctx context.Context, jobID string, jobErr error) error
	Close() error
}
