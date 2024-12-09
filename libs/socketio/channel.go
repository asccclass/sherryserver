package SherrySocketIO

import(
   "fmt"
   "sync"
   "time"
   "net/http"
   "math/rand"
   // "github.com/gorilla/mux"
   // "github.com/gorilla/websocket"
   // "github.com/asccclass/sherrytime"
)

// 房間/Channel
type RoomMap struct {
   Mutex sync.RWMutex
   Map map[string][]Client  // 連線人員
}

// 取得房間內的所有人
func(room *RoomMap) Get(roomID string)([]Client) {
   room.Mutex.RLock()
   defer room.Mutex.RUnlock()
   return room.Map[roomID]
}

// 刪除房間
func(room *RoomMap) DeleteRoom(roomID string) {
   room.Mutex.Lock()
   room.Mutex.Unlock()
   delete(room.Map, roomID)
}

// 加入房間
func(room *RoomMap) InsertIntoRoom(roomID string, client *Client) {
   room.Mutex.Lock()
   room.Mutex.Unlock()

   room.Map[roomID] = append(room.Map[roomID], *client)
}

func(room *RoomMap) Init() {  // Room 初始化
   room.Map = make(map[string][]Client)
}

// 建立房間(8位數亂數）
func(room *RoomMap) CreateRoom()(string) {
   room.Mutex.Lock()
   defer room.Mutex.Unlock()

   rand.Seed(time.Now().UnixNano())
   var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")

   b := make([]rune, 8)
   for i := range b {
      b[i] = letters[rand.Intn(len(letters))]
   }
   roomID := string(b)
   room.Map[roomID] = []Client{}
   return roomID 
}

// 建立 channel
func(io *SrySocketio) CreateChannel(w http.ResponseWriter, r *http.Request) {
   w.Header().Set("Access-Control-Allow-Origin", "*")
   roomID := io.AllRooms.CreateRoom()
   io.Message2Web(w, "roomID", fmt.Errorf(roomID))
}

// 加入channel
func(io *SrySocketio) JoinChannel(w http.ResponseWriter, r *http.Request) {
   roomID, ok := r.URL.Query()["roomID"]
   if !ok {
      io.Message2Web(w, "error", fmt.Errorf("no roomID"))
      return
   }
   ws, err := io.Upgrader.Upgrade(w, r, nil)
   if err != nil {
      io.Message2Web(w, "error", err)
      return
   }
   client := &Client{Conn: ws, Send: make(chan []byte, 1024), Host: true}  // 256
   io.AllRooms.InsertIntoRoom(roomID[0], client)
   io.Hub.Register <- client

   go client.WritePump(io.Hub)
   go client.ReadPumpInChannel(io.Hub, roomID[0])

   io.BroadCastMessageWs("{\"type\": \"WS_MESSAGE\", \"payload\": \"someone join " + roomID[0] + "\"}")
}
