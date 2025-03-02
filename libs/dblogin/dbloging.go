package DBLoginService

import(
   "os"
   "fmt"
   "strings"
   "net/http"
   "crypto/rand"
   "encoding/base64"
   "github.com/golang-jwt/jwt/v5"
   "github.com/asccclass/sherryserver"
)

type Login struct {
   HashedPassword	string
   SessionToken		string
   CSRFToken		string
}

type DBLoginService struct {
   Server *SherryServer.Server   // Server is the server that this middleware is attached to.
}

// Router 
func(app *Oauth2) AddRouter(router *http.ServeMux) {
   router.HandleFunc("POST /login/register", app.register)
   router.HandleFunc("POST /login", app.login)
}

// "email,profile"
func NewDBLoginService(server *SherryServer.Server) (*DBLoginService, error) {
   server.Logger.Info("Initial DB Login service ok")
   return &DBLoginService {
      Server: server,
   }, nil
}
