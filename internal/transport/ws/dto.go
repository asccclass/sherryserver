package ws

import "github.com/asccclass/sherryserver/internal/stt"

type ClientEnvelope struct {
	Type          string           `json:"type"`
	SessionID     string           `json:"sessionId,omitempty"`
	Setup         *stt.SessionSetup `json:"setup,omitempty"`
	Text          string           `json:"text,omitempty"`
	Audio         string           `json:"audio,omitempty"`
	FunctionReply []*FunctionReply `json:"functionReplies,omitempty"`
}

type FunctionReply struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Result map[string]any `json:"result,omitempty"`
	Error  string         `json:"error,omitempty"`
}

type ServerEnvelope struct {
	Type    string `json:"type"`
	Message any    `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}
