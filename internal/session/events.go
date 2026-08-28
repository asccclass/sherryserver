package session

type Event struct {
	Type      string         `json:"type"`
	SessionID string         `json:"sessionId,omitempty"`
	TurnID    string         `json:"turnId,omitempty"`
	Message   string         `json:"message,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
}
