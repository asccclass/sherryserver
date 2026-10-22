package model

import "encoding/json"

type LiveSentenceMessage struct {
	Text        string `json:"text"`
	Author      string `json:"author"`
	VideoID     string `json:"videoId"`
	MessageID   string `json:"messageId"`
	PublishedAt string `json:"publishedAt"`
}

func (m LiveSentenceMessage) Encode() ([]byte, error) {
	return json.Marshal(m)
}
