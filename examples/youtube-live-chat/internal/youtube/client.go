package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultBaseURL = "https://www.googleapis.com/youtube/v3"

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{BaseURL: defaultBaseURL, HTTPClient: httpClient}
}

type APIRequestError struct {
	Code    int
	Message string
	Reason  string
}

func (e *APIRequestError) Error() string {
	if e.Reason == "" {
		return fmt.Sprintf("youtube api error(code=%d): %s", e.Code, e.Message)
	}
	return fmt.Sprintf("youtube api error(code=%d, reason=%s): %s", e.Code, e.Reason, e.Message)
}

func IsLiveEnded(err error) bool {
	var apiErr *APIRequestError
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.Reason {
	case "liveChatEnded", "liveChatNotFound", "liveBroadcastNotFound", "videoNotFound":
		return true
	default:
		return false
	}
}

type liveBroadcastsResponse struct {
	Items []struct {
		Snippet struct {
			LiveChatID string `json:"liveChatId"`
			ResourceID struct {
				VideoID string `json:"videoId"`
			} `json:"resourceId"`
		} `json:"snippet"`
	} `json:"items"`
}

type liveChatMessagesResponse struct {
	NextPageToken         string `json:"nextPageToken"`
	PollingIntervalMillis int    `json:"pollingIntervalMillis"`
	Items                 []struct {
		ID            string `json:"id"`
		AuthorDetails struct {
			DisplayName string `json:"displayName"`
		} `json:"authorDetails"`
		Snippet struct {
			DisplayMessage string `json:"displayMessage"`
			PublishedAt    string `json:"publishedAt"`
		} `json:"snippet"`
	} `json:"items"`
}

type apiErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Errors  []struct {
			Reason string `json:"reason"`
		} `json:"errors"`
	} `json:"error"`
}

type LiveChatMessage struct {
	ID          string
	Author      string
	Text        string
	PublishedAt time.Time
}

type LiveChatPage struct {
	Messages        []LiveChatMessage
	NextPageToken   string
	PollingInterval time.Duration
}

func ParseLiveBroadcasts(body []byte, videoID string) (string, error) {
	var response liveBroadcastsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("parse liveBroadcasts response: %w", err)
	}
	for _, item := range response.Items {
		if item.Snippet.LiveChatID == "" {
			continue
		}
		if item.Snippet.ResourceID.VideoID == "" || item.Snippet.ResourceID.VideoID == videoID {
			return item.Snippet.LiveChatID, nil
		}
	}
	return "", fmt.Errorf("找不到 videoId=%s 對應的 activeLiveChatId", videoID)
}

func ParseLiveChatMessages(body []byte) (LiveChatPage, error) {
	var response liveChatMessagesResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return LiveChatPage{}, fmt.Errorf("parse liveChatMessages response: %w", err)
	}

	messages := make([]LiveChatMessage, 0, len(response.Items))
	for _, item := range response.Items {
		publishedAt, err := time.Parse(time.RFC3339, item.Snippet.PublishedAt)
		if err != nil {
			return LiveChatPage{}, fmt.Errorf("parse publishedAt: %w", err)
		}
		messages = append(messages, LiveChatMessage{
			ID:          item.ID,
			Author:      item.AuthorDetails.DisplayName,
			Text:        item.Snippet.DisplayMessage,
			PublishedAt: publishedAt,
		})
	}

	return LiveChatPage{
		Messages:        messages,
		NextPageToken:   response.NextPageToken,
		PollingInterval: time.Duration(response.PollingIntervalMillis) * time.Millisecond,
	}, nil
}

func (c *Client) ResolveActiveLiveChatID(ctx context.Context, apiKey, videoID string) (string, error) {
	endpoint := fmt.Sprintf("%s/liveBroadcasts?part=snippet&broadcastStatus=active&broadcastType=all&maxResults=50&key=%s", c.BaseURL, url.QueryEscape(apiKey))
	body, err := c.get(ctx, endpoint)
	if err != nil {
		return "", err
	}
	return ParseLiveBroadcasts(body, videoID)
}

func (c *Client) FetchLiveChatMessages(ctx context.Context, apiKey, liveChatID, pageToken string) (LiveChatPage, error) {
	query := url.Values{}
	query.Set("part", "snippet,authorDetails")
	query.Set("liveChatId", liveChatID)
	query.Set("key", apiKey)
	if pageToken != "" {
		query.Set("pageToken", pageToken)
	}
	endpoint := fmt.Sprintf("%s/liveChatMessages?%s", c.BaseURL, query.Encode())
	body, err := c.get(ctx, endpoint)
	if err != nil {
		return LiveChatPage{}, err
	}
	return ParseLiveChatMessages(body)
}

func (c *Client) get(ctx context.Context, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, parseAPIError(resp.StatusCode, body)
	}
	return body, nil
}

func parseAPIError(statusCode int, body []byte) error {
	var parsed apiErrorResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return &APIRequestError{Code: statusCode, Message: strings.TrimSpace(string(body))}
	}
	reason := ""
	if len(parsed.Error.Errors) > 0 {
		reason = parsed.Error.Errors[0].Reason
	}
	code := parsed.Error.Code
	if code == 0 {
		code = statusCode
	}
	return &APIRequestError{
		Code:    code,
		Message: parsed.Error.Message,
		Reason:  reason,
	}
}
