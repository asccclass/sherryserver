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
   TokenUrl string	// TokenURL is the URL to request a token.
   UserUrl string	// UserURL is the URL to request user information. 
   JwtKey string	// JwtKey is the key to use to sign the JWT.
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
// Access Token: {"ErrorCode":"invalid_request","Error":"Authorization Code expired"}
// Access Token: {"ErrorCode":"invalid_request","Error":"Authorization Code revoked"}
// 檢查是否有已經登入
func(app *Oauth2) Protect(next http.Handler) http.Handler { 
   return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {// 從 request 中讀取 session
		session, err := app.Server.SessionManager.Get(r, "fisaOauth")
      if err != nil {
         app.FISAAuthorize(w, r)    // 未登入，導向登入頁面
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

/*
// 從請求中獲取 JWT


claims := token.Claims.(jwt.MapClaims)
    tokenString := r.Header.Get("Authorization")
    if tokenString == "" {
        http.Error(w, "JWT missing in request header", http.StatusBadRequest)
        return
    }

    // 解析 JWT
    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        // 驗證 JWT
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return jwtKey, nil
    })

    if err != nil {
        http.Error(w, err.Error(), http.StatusUnauthorized)
        return
    }

    // 驗證 JWT 是否有效
    if !token.Valid {
        http.Error(w, "Invalid JWT", http.StatusUnauthorized)
        return
    }

    // 如果 JWT 有效，從 claims 中獲取用戶名
    if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
        data := claims["data"].(map[string]interface{})
        name := data["name"].(string)
        fmt.Fprintf(w, "Welcome %s!", username)
    } else {
        http.Error(w, "Failed to get username from JWT", http.StatusInternalServerError)
    }
*/    

func(app *Oauth2) AddRouter(router *http.ServeMux) {
   router.HandleFunc("GET /login/fisa", app.FISAAuthorize)
}

// "email,profile"
func NewOauth(server *SherryServer.Server) (*Oauth2, error) {
   endpoint := os.Getenv("EndPoint")
   clientID := os.Getenv("ClientID")
   clientSecret := os.Getenv("ClientSecret")
	redirectUri := os.Getenv("RedirectUri") // RedirectUri is the URL to redirect users going through the OAuth flow
   scope := os.Getenv("Scope")
   tokenUrl := os.Getenv("TokenUrl")
   userUrl := os.Getenv("UserUrl")
   jwtKey := os.Getenv("JwtKey")
   if endpoint == "" || clientID == "" || clientSecret == "" || redirectUri == "" || scope == "" || tokenUrl == "" || userUrl == "" || jwtKey == "" {
      return nil, fmt.Errorf("Missing required parameters")
   }
   // sps := strings.Split(scopes, ",")
   return &Oauth2{
		Server: server,
      ClientID: clientID,
      ClientSecret: clientSecret,
      Endpoint: endpoint,
      RedirectUri: redirectUri,
      Scopes: scope,
      TokenUrl: tokenUrl,
      UserUrl: userUrl,
      JwtKey: jwtKey,
   }, nil
}
