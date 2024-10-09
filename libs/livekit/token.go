package SryLiveKit

import(
   "fmt"
   "time"
   "net/http"
   "github.com/livekit/protocol/auth"
)

// 取得 Access Token
func(app *LiveKit) GetAccessToken(room, identity string)(string) {
   at := auth.NewAccessToken(app.APIKey, app.Secret)
   grant := &auth.VideoGrant{
      RoomJoin: true,
      Room: room,
   }
   at.AddGrant(grant). SetIdentity(identity).SetValidFor(time.Hour)
   token, _ := at.ToJWT()
   return token
}

// get access token from web, GET /{room}/token
func(app *LiveKit) GetAccessTokenFromWeb(w http.ResponseWriter, r *http.Request) {
   room := r.PathValue("room")
   name := r.PathValue("userID")

   w.Header().Set("Content-Type", "application/json;charset=UTF-8")
   w.WriteHeader(http.StatusOK)
   if room == "" || name == "" {
      fmt.Fprintf(w, "{\"status\": \"0\", \"message\":\"path is wrong\"}")
   }
   token := app.GetAccessToken(room, name)
   if token != "" {
      fmt.Fprintf(w, "{\"status\": \"0\", \"message\":\"Token is empty\"}")
      return
   }
   fmt.Fprintf(w, "{\"status\": \"1\", \"message\":\"\"}")
}
