package Ntfy

import(
   "io"
   "os"
   "fmt"
   "strings"
   "net/http"
   "crypto/rand"
   "encoding/base64"
   "github.com/asccclass/sherryserver"
)

type NotifyMsessage struct {
   To		string		`json:"to"`
   Message	string		`json:"message"`
   From		string		`json:"from"`
}

type Notify struct {
   Server *SherryServer.Server   // Server is the server that this middleware is attached to.
   ClientID  string	// ClientID is the application's ID.
}

func(app *Notify) Send(w http.ResponseWriter, r *http.Request) {
   w.WriteHeader(http.StatusOK)
   if err := r.ParseForm(); err != nil {
      fmt.Fprintf(w, "%s", err.Error())
      return
   }
   b, err := ioutil.ReadAll(r.Body)
   defer r.Body.Close()
   if err != nil {
      fmt.Fprintf(w, "Error: %s, Try post data.", err.Error())
      return
   }
   var msg NotifyMessage
   if err := json.Unmarshal(b, &msg); err != nil {
      fmt.Fprintf(w, "Error: %s, Try use post data.", err.Error())
      return
   }
   http.Post("https://ntfy.sh/mytopic", "text/plain", strings.NewReader(msg.Message)
}

// Router 
func(app *Notify) AddRouter(router *http.ServeMux) {
   router.HandleFunc("POST /ntfy/send", app.Send)
}

// "email,profile"
func NewNtfy(server *SherryServer.Server) (*Notify, error) {
   return &Notify{
      Server: server,
   }, nil
}
