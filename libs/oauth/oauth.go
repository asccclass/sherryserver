package Oauth

import(
   "io"
   "os"
   "fmt"
   //"strings"
   "net/http"
   "crypto/rand"
   "encoding/base64"
	"github.com/asccclass/sherryserver"
)

type Oauth2 struct {
	Server *SherryServer.Server   // Server is the server that this middleware is attached to.
   ClientID  string	// ClientID is the application's ID.
   ClientSecret string// ClientSecret is the application's secret.
   Endpoint string
   RedirectUri string	// RedirectURL is the URL to redirect users going through the OAuth flow
   Scopes string	// Scope specifies optional requested permissions []string{"email", "profile"},
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
// 
// 檢查是否有已經登入
func(app *Oauth2) Protect(next http.Handler) http.Handler { 
   return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {// 從 request 中讀取 session
		session, err := app.Server.SessionManager.Get(r, "fisaOauth")
      if err!= nil {
         app.FISAAuthorize(w, r)    // 未登入，導向登入頁面
         fmt.Println(err.Error())
         return
      }
      email, ok := session.Values["email"].(string)
      if !ok || email == "" {  
         code := r.URL.Query().Get("code")
         if code == "" {
            fmt.Println("未登入，導向登入頁面")
            app.FISAAuthorize(w, r)    // 未登入，導向登入頁面
            return
         } else {
            fmt.Println("登入成功，導向原本頁面")
            app.FISAAuthenticate(w, r, code) // 登入成功，導向原本頁面
            return
         }
         return
      } else {
         next.ServeHTTP(w, r)
      }
	})
}

func(app *Oauth2) AddRouter(router *http.ServeMux) {
   router.HandleFunc("GET /login/fisa", app.FISAAuthorize)
}

// "email,profile"
func NewOauth(server *SherryServer.Server, scopes string)(*Oauth2, error) {
   endpoint := os.Getenv("EndPoint")
   clientID := os.Getenv("ClientID")
   clientSecret := os.Getenv("ClientSecret")
	redirectUri := os.Getenv("RedirectUri") // RedirectUri is the URL to redirect users going through the OAuth flow
   if endpoint == "" || clientID == "" || clientSecret == "" || redirectUri == "" || scopes == "" {
      return nil, fmt.Errorf("Missing required parameters")
   }
   // sps := strings.Split(scopes, ",")
   return &Oauth2{
		Server: server,
      ClientID: clientID,
      ClientSecret: clientSecret,
      Endpoint: endpoint,
      RedirectUri: redirectUri,
      Scopes: scopes,
   }, nil
}
