package tts

import "context"

type HTTPClient struct{}

func (c *HTTPClient) Synthesize(ctx context.Context, req Request) (Response, error) {
	_ = ctx
	_ = req
	return Response{}, nil
}
