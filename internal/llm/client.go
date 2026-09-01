package llm

import "context"

type Prompt struct {
	System string `json:"system"`
	User   string `json:"user"`
}

type Reply struct {
	Text string `json:"text"`
}

type Client interface {
	Generate(ctx context.Context, prompt Prompt) (Reply, error)
}
