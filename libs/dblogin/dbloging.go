package DBLoginService

import(
   "net/http"
   "github.com/asccclass/sherryserver"
)

type Login struct {
   HashedPassword	string
   SessionToken		string
   CSRFToken		string
}

type DBLoginService struct {
   Server *SherryServer.Server   // Server is the server that this middleware is attached to.
   users	map[string]Login
}

// Router 
func(app *DBLoginService) AddRouter(router *http.ServeMux) {
   router.HandleFunc("POST /login/register", app.register)
   router.HandleFunc("POST /login", app.login)
}

// "email,profile"
func NewDBLoginService(server *SherryServer.Server) (*DBLoginService, error) {
   server.Logger.Info("Initial DB Login service ok")
   return &DBLoginService {
      Server: server,
      users: make(map[string]Login),
   }, nil
}
