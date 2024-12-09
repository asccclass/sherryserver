/*
   參考資訊：https://betterprogramming.pub/build-basic-real-time-competition-app-with-go-96c2ca0d35bf
*/
package SherrySocketIO

import(
   "fmt"
   "log"
   "time"
   "bytes"
   "github.com/gorilla/websocket" 
)

const (
   writeWait = 10 * time.Second
   pongWait = 60 * time.Second
   pingPeriod = (pongWait * 9) / 10
   maxMessageSize = 2048 //512
)

var (
   newline = []byte{'\n'}
   space = []byte{' ' }
)

type Client struct {
   Conn 	*websocket.Conn
   Send 	chan []byte
   Host		bool
}

// channel 使用
func(c *Client) ReadPumpInChannel(hub *SrySocketioHub, roomID string) {
   defer func() { // 結束，關閉client
      hub.Unregister <- c
      c.Conn.Close()
   }()
   c.Conn.SetReadLimit(maxMessageSize)
   c.Conn.SetReadDeadline(time.Now().Add(pongWait))
   c.Conn.SetPongHandler(func(string) error { c.Conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
   for {
      msg := &broadcastMsg {
         Client: c.Conn,
         RoomID: roomID,
      }
      if err := c.Conn.ReadJSON(&msg.Message); err != nil {
         if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
            log.Printf("error: %v", err)
         }
         fmt.Println("Read json eror:", err.Error())
         break
      }
      hub.Specific <- msg
   }
}

// 一般無channel使用
func(c *Client) ReadPump(hub *SrySocketioHub) {
   defer func() { // 結束，關閉client
      hub.Unregister <- c
      c.Conn.Close()
   }()
   c.Conn.SetReadLimit(maxMessageSize)
   c.Conn.SetReadDeadline(time.Now().Add(pongWait))
   c.Conn.SetPongHandler(func(string) error { c.Conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
   for {
      _, message, err := c.Conn.ReadMessage()
      if err != nil {
         if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
            log.Printf("error: %v", err)
         }
         break
      }
      message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
      hub.Broadcast <- message
   }
}

func (c *Client) WritePump(hub *SrySocketioHub) {
   ticker := time.NewTicker(pingPeriod)
   defer func() {
      ticker.Stop()
      c.Conn.Close()
   }()
   for {
      select {
         case message, ok := <-c.Send:
            c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
            if !ok {
               c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
               return
            }
            w, err := c.Conn.NextWriter(websocket.TextMessage)
            if err != nil {
               return
            }
            w.Write(message)
            n := len(c.Send)
            for i := 0; i < n; i++ {
               // w.Write(newline)  // 不輸出\n，避免破壞原始資料
               w.Write(<-c.Send)
            }
            if err := w.Close(); err != nil {
               return
            }
         case <-ticker.C:
            c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
            if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
               return
            }
      }
   }
}
