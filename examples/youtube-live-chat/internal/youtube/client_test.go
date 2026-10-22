package youtube

import (
	"testing"
	"time"
)

func TestParseLiveBroadcasts(t *testing.T) {
	body := []byte(`{
		"items": [
			{"snippet": {"liveChatId": "chat-other", "resourceId": {"videoId": "other-video"}}},
			{"snippet": {"liveChatId": "chat-123", "resourceId": {"videoId": "target-video"}}}
		]
	}`)

	chatID, err := ParseLiveBroadcasts(body, "target-video")
	if err != nil {
		t.Fatalf("ParseLiveBroadcasts failed: %v", err)
	}
	if chatID != "chat-123" {
		t.Fatalf("unexpected chat id: %s", chatID)
	}
}

func TestParseLiveChatMessages(t *testing.T) {
	body := []byte(`{
		"nextPageToken": "NEXT_TOKEN",
		"pollingIntervalMillis": 1750,
		"items": [
			{
				"id": "msg-1",
				"authorDetails": {"displayName": "bob"},
				"snippet": {
					"displayMessage": "hello world",
					"publishedAt": "2026-10-22T00:00:00Z"
				}
			}
		]
	}`)

	page, err := ParseLiveChatMessages(body)
	if err != nil {
		t.Fatalf("ParseLiveChatMessages failed: %v", err)
	}
	if page.NextPageToken != "NEXT_TOKEN" {
		t.Fatalf("unexpected next page token: %s", page.NextPageToken)
	}
	if page.PollingInterval != 1750*time.Millisecond {
		t.Fatalf("unexpected polling interval: %v", page.PollingInterval)
	}
	if len(page.Messages) != 1 {
		t.Fatalf("unexpected message count: %d", len(page.Messages))
	}
	if page.Messages[0].ID != "msg-1" || page.Messages[0].Author != "bob" || page.Messages[0].Text != "hello world" {
		t.Fatalf("unexpected parsed message: %+v", page.Messages[0])
	}
}

func TestIsLiveEnded(t *testing.T) {
	err := &APIRequestError{Code: 403, Message: "The live chat is no longer live.", Reason: "liveChatEnded"}
	if !IsLiveEnded(err) {
		t.Fatal("expected IsLiveEnded to return true")
	}
}
