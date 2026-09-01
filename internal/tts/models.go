package tts

type Request struct {
	SessionID string `json:"sessionId"`
	TurnID    string `json:"turnId"`
	Text      string `json:"text"`
	Voice     string `json:"voice"`
	Format    string `json:"format"`
}

type Response struct {
	AudioURL   string `json:"audioUrl"`
	DurationMS int    `json:"durationMs"`
}
