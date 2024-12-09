package SherrySocketIO

import(
   "fmt"
   "sync"
   "time"
   "net/http"
   "math/rand"
)

type room struct {
   clients	map[*Client]bool
   join		chan	*Client
   leave	chan	*Client
   forward	chan	[]byte
}

func(r *room) run() {
   for {
      select {
         case client := <-r.join:
            r.clients[client] = true
	 case client := <-r.leave:
            delete(r.clients, client)
	    close(client)
        case msg := <-r.forward:
           for client := range r.clients {
              client.receive <- msg
	   }
      }
   }
}

func newRoom()(*room) {
   return &room {
      clients: make(map[*Client]bool),
      join: make(chan *Client),
      leave: make(chan *Client),
      forward: make(chan []byte)
   }
}
