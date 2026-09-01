package tts

import "context"

type Client interface {
	Synthesize(ctx context.Context, req Request) (Response, error)
}
