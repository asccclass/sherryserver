package SherrySocketIO

import(
   "fmt"
   "time"
   "context"
   "net/http"
   "math/rand"
)

// 處理 /sse
func(app *SrySocketio) ListenSSE(w http.ResponseWriter, r *http.Request) {
   flusher, ok := w.(http.Flusher)
   if !ok {
      http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
      return
   }
   w.Header().Set("Content-Type", "text/event-stream")
   w.Header().Set("Cache-Control", "no-cache")
   w.Header().Set("Connection", "keep-alive")
   // w.Header().Set("Access-Control-Allow-Origin", "*") // For development
   authKey := fmt.Sprintf("client-%d-%d", time.Now().UnixNano(), rand.Intn(10000))
   client := &Client{
      id:      authKey,
      channel: make(chan []byte, 10), // Buffered channel
      req:     r,
   }
   s.addClient <- client

   defer func() {
      s.delClient <- client
   }()
   pingTicker := time.NewTicker(20 * time.Second)
   defer pingTicker.Stop()
   for {
      select {
      case <-r.Context().Done(): // Client disconnected
         return
      case msg, open := <-client.channel:
         if !oopen {
            return
	 }
	 if _, err := w.Write(msg); err != nil {
            fmt.Printf("Error writing to client %s: %v", client.id, err)
	    return
	 }
	 flusher.Flush()
      case <-pingTicker.C:
         pingMsg := MCPMessage{
            MCPMessageName: "dns-com-awns-ping",
	    MessageID:      fmt.Sprintf("server-ping-%d", time.Now().UnixNano()),
	    AuthKey:        client.id, // Send auth key with ping
	 }
	 jsonData, _ := json.Marshal(pingMsg)
	 sseEvent := fmt.Sprintf("event: mcp_ping\ndata: %s\n\n", string(jsonData))
	 if _, err := w.Write([]byte(sseEvent)); err != nil {
            fmt.Printf("Error sending ping to client %s: %v", client.id, err)
	    return
	 }
         flusher.Flush()
      }
   }
}
