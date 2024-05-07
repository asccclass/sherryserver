package Oauth

import(
   "os"
   "strings"
   "net/http"
)

type Oauth2 struct {
   ClientID [string]	// ClientID is the application's ID.
   ClientSecret [string]// ClientSecret is the application's secret.
   Endpoint [string]
   RedirectUri [string]	// RedirectURL is the URL to redirect users going through the OAuth flow
   Scopes [][string]	// Scope specifies optional requested permissions []string{"email", "profile"},
}

// http.Redirect(w, r, url, http.StatusTemporaryRedirect)
// code := r.URL.Query().Get("code")
func googleHandler(w http.ResponseWriter, r *http.Request) {
   // url := Oauth2.AuthCodeURL("state", oauth2.AccessTypeOffline)
   // http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func AddRouter(router *mux.Router) {
   // router.HandleFunc("GET /auth/login/google", googleHandler)
}

func NewOauth(redirectUri, scopes string)(*Oauth2, error) {
   endpoint := os.Getenv("EndPoint")
   clientID := os.Getenv("ClientID")
   clientSecret := os.Getenv("ClientSecret")
   if endpoint == "" || clientID == "" || clientSecret == "" || redirectUri == "" || scopes == "" {
      return nil, errors.New("Missing required parameters")
   }
   sps := strings.Split(scopes, ",")
   return &Oauth2{
      ClientID: clientID,
      ClientSecret: clientSecret,
      Endpoint: "https://api.twitter.com/oauth2/token",
      RedirectUri: redirectUri,
      Scopes: sps,
   }, nil
}
