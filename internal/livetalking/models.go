package livetalking

type PublishTarget struct {
	Target      string `json:"target"`
	Room        string `json:"room"`
	Participant string `json:"participant"`
}

type AudioSource struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type JobRequest struct {
	JobID     string        `json:"jobId"`
	SessionID string        `json:"sessionId"`
	TurnID    string        `json:"turnId"`
	AvatarID  string        `json:"avatarId"`
	Audio     AudioSource   `json:"audio"`
	Publish   PublishTarget `json:"publish"`
}

type JobResponse struct {
	OK    bool   `json:"ok"`
	JobID string `json:"jobId"`
	State string `json:"state"`
}
