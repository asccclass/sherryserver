package SherrySocketIO

import(
   "fmt"
   "time"
   "sync"
   "context"
   "net/http"
   "io/ioutil"
   "encoding/json"
   "github.com/coder/websocket"
   "github.com/asccclass/sherrytime"
)

type SrySocketio struct {
   subscriberMessageBuffer int
   mux                     http.ServeMux
   subscribersMu           sync.Mutex
   subscribers             map[*subscriber]struct{}
}

type subscriber struct {
   msgs chan []byte
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

// 送出訊息給特定的人
// func(app *SrySocketio) ToSpecificMessage() {
// }

// 送出訊息給全部的Client
func(app *SrySocketio) BroadCastMessageWs(message []byte) {
   app.subscribersMu.Lock()
   defer app.subscribersMu.Unlock()

   for s := range app.subscribers {
      s.msgs <- message
   }
}

// 送出訊息給特定的channel
func(app *SrySocketio) BroadcastMessage2Channel(channel, message []byte) {
}

// 格式化訊息
func(app *SrySocketio) TransMessagePackageToJson(messagePackage *MessagePackage) {
   if messagePackage.TimeStamp == "" {
      st := sherrytime.NewSherryTime("Asia/Taipei", "-")  // Initial
      messagePackage.TimeStamp = st.Now()
   }
   byteArray, _ := json.Marshal(messagePackage)  // 格式化輸出
   app.BroadCastMessageWs(byteArray)
}

// 加入訂閱者
func(app *SrySocketio) addSubscriber(subscriber *subscriber) {
   app.subscribersMu.Lock()
   app.subscribers[subscriber] = struct{}{}
   app.subscribersMu.Unlock()
   fmt.Println("Added subscriber", subscriber)
}

// 處理 /ws 
func(app *SrySocketio) Listen(w http.ResponseWriter, r *http.Request) {
   var c *websocket.Conn
   subscriber := &subscriber{
      msgs: make(chan []byte, app.subscriberMessageBuffer),
   }
   app.addSubscriber(subscriber)

   ctx := r.Context()
   c, err := websocket.Accept(w, r, nil)
   if err != nil {
      fmt.Println(err)
      return
   }
   defer c.CloseNow()
   ctx = c.CloseRead(ctx)
   for {
      select {
      case msg := <-subscriber.msgs:
         ctx, cancel := context.WithTimeout(ctx, time.Second*5)
         defer cancel()
         if err := c.Write(ctx, websocket.MessageText, msg); err != nil {
            fmt.Println(err)
            return
         }
      case <-ctx.Done():
         fmt.Println(err)
         return
      }
   }
}

// 處理 /sendmessage
func(app *SrySocketio) SendMessageInString(w http.ResponseWriter, r *http.Request) {
   b, err := ioutil.ReadAll(r.Body)
   defer r.Body.Close()
   if err != nil {
      fmt.Println(err)
      return
   }
   app.BroadCastMessageWs(b)
}

// Router
func(app *SrySocketio) AddRouter(router *http.ServeMux) {
   // app.subscribersMu = router
   router.HandleFunc("/ws", app.Listen)
   // router.HandleFunc("/sse", app.ListenSSE)
   router.Handle("POST /sendsocketmessageinstring", http.HandlerFunc(app.SendMessageInString))
/*
   router.Handle("/create/channel/{name}", http.HandlerFunc(app.CreateChannel))
   router.Handle("POST /sendsocketmessageinjson", http.HandlerFunc(app.SendMessageInJson))
   router.Handle("GET /createroom", http.HandlerFunc(app.CreateChannel))
   router.Handle("GET /joinroom", http.HandlerFunc(app.JoinChannel))
*/
}

// 初始化SrySocketio
func NewSrySocketio()(*SrySocketio) {
   return &SrySocketio {
      subscriberMessageBuffer: 10,
      subscribers:             make(map[*subscriber]struct{}),
   }
}
