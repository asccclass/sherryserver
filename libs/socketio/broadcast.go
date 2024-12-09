package SherrySocketIO

import(
   "github.com/gorilla/websocket"
)

type broadcastMsg struct {
   Message	map[string]interface{}
   RoomID	string
   Client	*websocket.Conn 
}
