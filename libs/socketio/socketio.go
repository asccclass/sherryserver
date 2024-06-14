// 參考資料：https://www.youtube.com/watch?v=JuUAEYLkGbM
package SherrySocketIO

import(
	"io"
	"net/http"
	"golang.org/x/net/websocket"
)

type SocketServer struct {
	conns map[*websocket.Conn]bool
	broadcast chan []byte
}

func(app *SocketServer) Run() {
	for {
		select {
		case msg := <-app.broadcast:
			for conn := range app.conns {
				websocket.Message.Send(conn, string(msg))
			}
		}
	}
}

func(app *SocketionServer) readLoop(ws *websocket.Conn) {
	buf := make([]byte, 1024)
	for {
		n, err := ws.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			continue
		}
		app.broadcast <- buf[:n]
	}
}

// 處理連線
func(app *SocketServer) handleWS(ws *websocket.Conn) {
   app.conns[ws] = true
	app.readLoop(ws)
}

// Router 
func(app *SocketServer) AddRouter(router *http.ServeMux) {
   router.HandleFunc("/ws", app.handleWS)
}

func NewSocketServer() (*SocketServer) {
	return &SocketServer{
		conns: make(map[*websocket.Conn]bool),
		broadcast: make(chan []byte, 256),
	}
}
