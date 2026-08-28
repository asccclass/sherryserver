package app

import "net/http"

type Server struct {
	HTTPServer *http.Server
}

func NewServer(handler http.Handler, addr string) *Server {
	return &Server{
		HTTPServer: &http.Server{
			Addr:    addr,
			Handler: handler,
		},
	}
}
