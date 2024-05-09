package Oauth

import(
   "os"
   "strings"
   "net/http"
   "encoding/base64"
	"github.com/asccclass/sherryserver"
)

type Oauth2 struct {
	Server *sherryserver.Server   // Server is the server that this middleware is attached to.
   ClientID [string]	// ClientID is the application's ID.
   ClientSecret [string]// ClientSecret is the application's secret.
   Endpoint [string]
   RedirectUri [string]	// RedirectURL is the URL to redirect users going through the OAuth flow
   Scopes [][string]	// Scope specifies optional requested permissions []string{"email", "profile"},
}

// state参数用於防止CSRF（Cross site attack)  傳入長度，通常32
func(app *Oauth2) State(n int) (string, error) {
	data := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, data); err != nil {
		 return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// http.Redirect(w, r, url, http.StatusTemporaryRedirect)
// code := r.URL.Query().Get("code")
func(app *Oauth2) googleHandler(w http.ResponseWriter, r *http.Request) {
   // url := Oauth2.AuthCodeURL("state", oauth2.AccessTypeOffline)
   // http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func(app *Oauth2) AddRouter(router *mux.Router) {
   router.HandleFunc("GET /login/fisa", app.FISALogin)
   router.HandleFunc("GET /callback/fisa", app.FISACallback)
}

// "email,profile"
func NewOauth(server *SherryServer.Server, scopes string)(*Oauth2, error) {
   endpoint := os.Getenv("EndPoint")
   clientID := os.Getenv("ClientID")
   clientSecret := os.Getenv("ClientSecret")
	redirectUri := os.Getenv("RedirectUri") // RedirectUri is the URL to redirect users going through the OAuth flow
   if endpoint == "" || clientID == "" || clientSecret == "" || redirectUri == "" || scopes == "" {
      return nil, errors.New("Missing required parameters")
   }
   sps := strings.Split(scopes, ",")
   return &Oauth2{
		Server: server,
      ClientID: clientID,
      ClientSecret: clientSecret,
      Endpoint: "https://api.twitter.com/oauth2/token",
      RedirectUri: redirectUri,
      Scopes: sps,
   }, nil
}
