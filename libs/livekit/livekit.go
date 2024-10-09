package SryLiveKit

import(
   "os"
   "fmt"
   "net/http"
   lksdk "github.com/livekit/server-sdk-go"
)

type LiveKit struct {
   APIKey	string		`json:"APIKey"`
   Secret	string		`json:"secret"`
   RoomManager	lksdk.*RoomServiceClient
}

// Router
func(app *LiveKit) AddRouter(router *http.ServeMux) {
   router.HandleFunc("GET /{room}/token/{userID}", app.GetAccessTokenFromWeb)
}

func NewSryLiveKit()(*LiveKit, error) {
   url := os.Getenv("HOST")
   key := os.Getenv("LIVEKIT_API_KEY")
   secret := os.Getenv("LIVEKIT_API_SECRET")

   if key == "" || secret == "" || url == "" {
      return nil, fmt.Errorf("No api key or secret key")
   }

   roomClient := lksdk.NewRoomServiceClient(url, key, secret)

   return &LiveKit {
      APIKey: key,
      Secret: secret,
      RoomManager: roomClient,
   }, nil
}
