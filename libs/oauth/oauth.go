package Oauth

import(
   "io"
   "os"
   "fmt"
   "strings"
   "net/http"
   "crypto/rand"
   "encoding/base64"
	"github.com/golang-jwt/jwt/v5"
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

// 取得個人資料 from Authorization Code
func(app *Oauth2) GetUserInfoFromJWT(tokenString string) (map[string]interface{}, error) {
   userinfo := make(map[string]interface{})
   token, err := app.GetJWTToken(tokenString)
   if err!= nil {
      return userinfo, err
   }
   claims, ok := token.Claims.(jwt.MapClaims)
   if !ok || !token.Valid {
      return userinfo, err
   }
   userinfo = claims["data"].(map[string]interface{})
   // name := data["name"].(string)
   return userinfo, nil
}

func(app *Oauth2) GetJWTToken(tokenString string) (*jwt.Token, error) {
   if tokenString == "" {
      return nil, fmt.Errorf("JWT missing in request header")
   }
   token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {  // 解析 JWT
      if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {  // 驗證 JWT
         return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
      }
      return app.JwtKey, nil
   })
   if err != nil {
      return nil, fmt.Errorf("JWT missing in request header")
   }
   // 驗證 JWT 是否有效
   if !token.Valid {
      return nil, fmt.Errorf("Invalid JWT")
   }
   if _, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
      return token, nil
   } 
   return nil, fmt.Errorf("Failed to get username from JWT")   
}

func(app *Oauth2) IsValidJWT(r *http.Request) (error) {
   str := r.Header.Get("Authorization")
   if str == "" {
      return fmt.Errorf("JWT missing in request header")
   }
   s := strings.Split(str, " ")
   if len(s) != 2 || s[0] != "Bearer" {
      return fmt.Errorf("Invalid Authorization header")
   }
   _, err := app.GetJWTToken(s[1])
   return err
}

// 從 r 取得個人資料
func(app *Oauth2) GetUserInfoFromRequest(r *http.Request) (map[string]interface{}, error) {
   s := strings.Split(r.Header.Get("Authorization"), " ")
   if len(s) != 2 || s[0] != "Bearer" {
      return nil, fmt.Errorf("Invalid Authorization header")
   }
   return app.GetUserInfoFromJWT(s[1])
}

// http.Redirect(w, r, url, http.StatusTemporaryRedirect)
// Access Token: {"ErrorCode":"invalid_request","Error":"Authorization Code expired"}
// Access Token: {"ErrorCode":"invalid_request","Error":"Authorization Code revoked"}
// 檢查是否有已經登入
func(app *Oauth2) Protect(next http.Handler) http.Handler { 
   return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {// 從 request 中讀取 session
		session, err := app.Server.SessionManager.Get(r, "fisaOauth")
      if err != nil {
         fmt.Println("未登入，導向登入頁面")
         app.FISAAuthorize(w, r)    // 未登入，導向登入頁面
         return
      }
      email, ok := session.Values["email"].(string)
      if !ok || email == "" {  
         code := r.URL.Query().Get("code")
         if code == "" {
            app.FISAAuthorize(w, r)    // 未登入，導向登入頁面
            return
         } else {
            fmt.Println("登入成功，導向原本頁面")
            app.FISAAuthenticate(w, r, code) // 登入成功，導向原本頁面
            next.ServeHTTP(w, r)
         }
         return
      } else {
         if err := app.IsValidJWT(r); err != nil {
            fmt.Println("JWT 失效，導向登入頁面", err.Error())
            app.FISAAuthorize(w, r)    // JWT 失效，導向登入頁面
            return
         }
         fmt.Println("已經登入，導向原本頁面")
         next.ServeHTTP(w, r)
      }
	})
}

// Router 
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
