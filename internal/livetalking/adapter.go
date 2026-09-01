package livetalking

import "context"

type HTTPClient struct{}

func (c *HTTPClient) StartJob(ctx context.Context, req JobRequest) (JobResponse, error) {
	_ = ctx
	_ = req
	return JobResponse{}, nil
}

func (c *HTTPClient) CancelJob(ctx context.Context, jobID string) error {
	_ = ctx
	_ = jobID
	return nil
}
