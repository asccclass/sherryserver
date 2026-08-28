package livetalking

import "context"

type Client interface {
	StartJob(ctx context.Context, req JobRequest) (JobResponse, error)
	CancelJob(ctx context.Context, jobID string) error
}
