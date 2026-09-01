package session

import "time"

type State string

const (
	StateIdle        State = "idle"
	StateListening   State = "listening"
	StateThinking    State = "thinking"
	StateTTS         State = "tts"
	StateRendering   State = "rendering"
	StateStreaming   State = "streaming"
	StateInterrupted State = "interrupted"
	StateError       State = "error"
)

type Session struct {
	ID              string
	UserID          string
	State           State
	RecognizedText  string
	SystemPrompt    string
	LastError       string
	LLMReply        string
	LiveTalkingJobID string
	LiveKitRoom     string
	CurrentTurnID   string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
