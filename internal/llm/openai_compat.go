package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type OpenAICompatConfig struct {
	APIURL       string
	APIKey       string
	Model        string
	SystemPrompt string
	Timeout      time.Duration
}

type OpenAICompatClient struct {
	cfg        OpenAICompatConfig
	httpClient *http.Client
}

type openAIChatRequest struct {
	Model       string              `json:"model,omitempty"`
	Messages    []openAIChatMessage `json:"messages"`
	Temperature float32             `json:"temperature,omitempty"`
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func NewOpenAICompatClient(cfg OpenAICompatConfig) *OpenAICompatClient {
	return &OpenAICompatClient{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout + (5 * time.Second),
		},
	}
}

func (c *OpenAICompatClient) Generate(ctx context.Context, prompt Prompt) (Reply, error) {
	if strings.TrimSpace(c.cfg.APIURL) == "" {
		return Reply{Text: fmt.Sprintf("我收到你的需求了：%s", prompt.User)}, nil
	}

	if c.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.cfg.Timeout)
		defer cancel()
	}

	systemPrompt := MergeSystemPrompts(strings.TrimSpace(c.cfg.SystemPrompt), strings.TrimSpace(prompt.System))
	if systemPrompt == "" {
		systemPrompt = "You are a helpful assistant. Answer clearly and briefly in Traditional Chinese."
	}

	reqBody := openAIChatRequest{
		Model: c.cfg.Model,
		Messages: []openAIChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: prompt.User},
		},
		Temperature: 0.3,
	}

	raw, err := json.Marshal(reqBody)
	if err != nil {
		return Reply{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.APIURL, bytes.NewReader(raw))
	if err != nil {
		return Reply{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(c.cfg.APIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Reply{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Reply{}, err
	}
	if resp.StatusCode >= 300 {
		return Reply{}, fmt.Errorf("llm http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed openAIChatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Reply{}, err
	}
	if len(parsed.Choices) == 0 {
		return Reply{}, fmt.Errorf("llm returned no choices")
	}

	reply := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if reply == "" {
		return Reply{}, fmt.Errorf("llm returned empty content")
	}

	return Reply{Text: reply}, nil
}
