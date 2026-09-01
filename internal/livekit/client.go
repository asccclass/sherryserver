package livekit

import "context"

type Client interface {
	CreateRoom(ctx context.Context, sessionID string) (RoomInfo, error)
	CreateIngress(ctx context.Context, room string) (IngressInfo, error)
}
