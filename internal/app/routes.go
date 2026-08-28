package app

import "net/http"

func NewMux() *http.ServeMux {
	return http.NewServeMux()
}
