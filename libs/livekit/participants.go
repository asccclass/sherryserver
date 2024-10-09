// https://docs.livekit.io/home/server/managing-rooms/
package SryLiveKit

import(
   "context"
   livekit "github.com/livekit/protocol/livekit"
)

// Create room
func(app *LiveKit) CreateRoom(roomName string, maxParticipants int)(*livekit.Room, error) {
   return app.RoomManager.CreateRoom(context.Background(), &livekit.CreateRoomRequest{
      Name: roomName,
      EmptyTimeout:    10 * 60, // 10 minutes
      MaxParticipants: maxParticipants,
   })
}

// Read room list
func(app *LiveKit) ListRooms()([]*livekit.Room, error){
   return app.RoomManager.ListRooms(context.Background(), &livekit.ListRoomsRequest{})
}

// Delete room
func(app *LiveKit) UpdateRoom(roomName string) {
   _, _ = roomClient.DeleteRoom(context.Background(), &livekit.DeleteRoomRequest{
     Room: roomName,
   })
}
