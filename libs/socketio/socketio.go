package SherrySocketIO

import(
   "os"
   "fmt"
   "time"
   "strings"
   "net/http"
   "encoding/json"
   "github.com/gorilla/websocket" 
   "github.com/asccclass/sherrytime"
   "github.com/asccclass/foldertree"
)

type SrySocketioHub struct {
   Clients	map[*Client]bool
   Broadcast	chan []byte
   Register	chan *Client
   Unregister	chan *Client
   Specific	chan *broadcastMsg
}

type SrySocketio struct {
   Hub		*SrySocketioHub
   Upgrader	websocket.Upgrader
   AllRooms	RoomMap
   Logfile	string
}

// HTTP log message
type HTTPMessagePackage struct {
   RemoteAddr   string          `json:"remoteAddr,omitempty"`
   Method       string          `json:"method,omitempty"`
   Status       string          `json:"status,omitempty"`
   Proto        string          `json:"proto,omitempty"`
   Url          string          `json:"url,omitempty"`
   Bytes        string          `json:"bytes,omitempty"`
}

type MessagePackage struct {
   Action       string                  `json:"action"`
   TimeStamp    string                  `json:"timestamp"`
   Message      string                  `json:"message"`
   To		string			`json:"to"`
   Http         *HTTPMessagePackage     `json:"http,omitempty"`
// PersonInfo   *Person                 `json:"person,omitempty"`
}

// Heart Beat
func(app *SrySocketio) Heartbeat(clnt *Client) {
   ticker := time.NewTicker(10 * time.Second)
   defer ticker.Stop()
   for {
      select {
         case <-ticker.C:
            if err := clnt.Conn.WriteMessage(websocket.PingMessage, []byte("heartbeat")); err != nil {
               return
	    }
	    clnt.Conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	    _, _, err := clnt.Conn.ReadMessage()
	    if err != nil {
               // ToDo 重新連線 or 用戶離線
               fmt.Println(err.Error())
	       delete (app.Hub.Clients, clnt)
	       return
	    }
      }
   }
}

// 執行go function
func(app *SrySocketio) Run() {
   ticker := time.NewTicker(5 * time.second)
   defer ticker.Stop()

   for {
      select {
         case client := <-app.Hub.Register:		// 註冊  onConnection
            app.Hub.Clients[client] = true
         case client := <-app.Hub.Unregister:		// 離開  onClose
            if _, ok := app.Hub.Clients[client]; ok {
               delete(app.Hub.Clients, client)
               close(client.Send)
            }
         case message := <-app.Hub.Specific:		// 傳送訊息給特定channel全部的人
            for _, client := range app.AllRooms.Map[message.RoomID] {  // client is broadcastMsg
               if(client.Conn != message.Client) {  // 不用送給自己
                  if err := client.Conn.WriteJSON(message.Message); err != nil {
                     fmt.Println(err.Error())
                     client.Conn.Close()
                  } 
               }
            } 
         case message := <-app.Hub.Broadcast:		// 傳送訊息給全部人
            for client := range app.Hub.Clients {
               select {
                  case client.Send <- message:
                  default:
                     close(client.Send)
                     delete(app.Hub.Clients, client)
               }
            }
      }
   }
}

// 送出訊息給特定的人
func(app *SrySocketio) ToSpecificMessage(data JsonMsg) {
   if data.To == ""  {
      app.Hub.Broadcast <-[]byte(data.Message)
   } else {
      app.Hub.Broadcast <-[]byte(data.Message)
/*
      for client := range io.Hub.Clients {
      }
*/
   }
}

// 送出訊息給全部的Client
func(app *SrySocketio) BroadCastMessageWs(message string) {
   app.Hub.Broadcast <-[]byte(message)
}

// 送出訊息給特定的channel
func(app *SrySocketio) BroadcastMessage2Channel(channel, message string) {
}

// 格式化訊息
func(app *SrySocketio) TransMessagePackageToJson(messagePackage *MessagePackage) {
   if messagePackage.TimeStamp == "" {
      st := sherrytime.NewSherryTime("Asia/Taipei", "-")  // Initial
      messagePackage.TimeStamp = st.Now()
   }
   byteArray, _ := json.Marshal(messagePackage)  // 格式化輸出
   app.BroadCastMessageWs(string(byteArray))
}

// 初始化留言區內容
func(app *SrySocketio) initialRead(client *Client, n int) {
   trees, err := foldertree.NewSryDocument("linux", os.Getenv("markdownFolder"), false)
   if err != nil {
      fmt.Println(err.Error())
      return
   }
   lines, err := trees.ReadLastNLines(app.Logfile, 20)
   if err != nil {
      fmt.Println(err.Error())
      return
   }
   client.Send <- []byte(strings.Join(lines, "\n"))
}

// 處理 /ws 
func(app *SrySocketio) Listen(w http.ResponseWriter, r *http.Request) {
   ws, err := app.Upgrader.Upgrade(w, r, nil)
   if err != nil {
      st := sherrytime.NewSherryTime("Asia/Taipei", "-")  // Initial
      fmt.Fprintf(w, "{\"status\": \"failure\", \"message\": \"" + err.Error() + "\", \"timestamp\":\"" + st.Now() + "\"}")
      return
   }
   client := &Client{Conn: ws, Send: make(chan []byte, 1024)}  // 256
   app.Hub.Register <- client

   // 讀取log檔 參考：https://github.com/DrkCoater/go-tail-f-follows/blob/main/main.go#L211
   if app.Logfile != "" {
      go app.initialRead(client, 20)   // 讀取最後 20 行資料
   }

   go app.Heartbeat(client)
   go client.WritePump(app.Hub)
   go client.ReadPump(app.Hub)

   app.BroadCastMessageWs("{\"type\": \"WS_MESSAGE\", \"payload\": \"someone connected\"}")
}

// Router
func(app *SrySocketio) AddRouter(router *http.ServeMux) {
   router.Handle("/ws", http.HandlerFunc(app.Listen))
   router.Handle("/create/channel/{name}", http.HandlerFunc(app.CreateChannel))
   router.Handle("POST /sendsocketmessageinjson", http.HandlerFunc(app.SendMessageInJson))
   router.Handle("POST /sendsocketmessageinstring", http.HandlerFunc(app.SendMessageInString))
   router.Handle("GET /createroom", http.HandlerFunc(app.CreateChannel))
   router.Handle("GET /joinroom", http.HandlerFunc(app.JoinChannel))
}

// 初始化SrySocketio
func NewSrySocketio()(*SrySocketio) {
   upgrader := websocket.Upgrader{ // StartSocketio
      ReadBufferSize:  1024,
      WriteBufferSize: 1024,
   }
   hub := &SrySocketioHub {			// *Broadcaster
      Clients:    make(map[*Client]bool),	// clients:    make(map[*Client]bool),
      Broadcast:  make(chan []byte),		// broadcast:  make(chan string),
      Register:   make(chan *Client),		// register:   make(chan *Client),
      Unregister: make(chan *Client),		// unregister: make(chan *Client),
      Specific:   make(chan *broadcastMsg),
   }
   var m RoomMap
   m.Init()
   ssio := &SrySocketio {
      Hub: hub,
      Upgrader: upgrader,
      AllRooms: m,
      Logfile: os.Getenv("socketiologfile"),
   }
   ssio.Run()     // 啟動監聽
   return ssio
}
