package livekit

type IngressInfo struct {
	ID        string `json:"id"`
	RTMPURL   string `json:"rtmpUrl"`
	StreamKey string `json:"streamKey"`
	WHIPURL   string `json:"whipUrl"`
}
