package model

import (
	"encoding/json"
	"testing"
)

func TestLiveSentenceMessageEncode(t *testing.T) {
	msg := LiveSentenceMessage{
		Text:        "hello",
		Author:      "alice",
		VideoID:     "video-1",
		MessageID:   "msg-1",
		PublishedAt: "2026-10-22T00:00:00Z",
	}
	bytes, err := msg.Encode()
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	var decoded map[string]string
	if err := json.Unmarshal(bytes, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if decoded["text"] != "hello" || decoded["author"] != "alice" || decoded["videoId"] != "video-1" || decoded["messageId"] != "msg-1" || decoded["publishedAt"] != "2026-10-22T00:00:00Z" {
		t.Fatalf("unexpected encoded json: %v", decoded)
	}
}
