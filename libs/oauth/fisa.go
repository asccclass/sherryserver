/*
   中央研究院 FISA SSO 服務介面
	// 
*/
package Oauth

import(
   // "os"
   // "io"
   "fmt"
   "time"
   "net/url"
   "net/http"
   "io/ioutil"
   "encoding/json"
   "github.com/golang-jwt/jwt/v5"
)

type AccessToken struct {
   AccessToken string `json:"access_token"`
   TokenType string `json:"token_type"`
   ExpiresIn int `json:"expires_in"`
   Scope string `json:"scope"`
   RefreshToken string `json:"refresh_token"`
   Error string `json:"error"`
   ErrorDescription string `json:"error_description"`
}

// "cn":"andyliu","chName":"OOO","phone":"02-27899963","email":"andyliu@gate.sinica.edu.tw","instCode":"24","sysid":"119511"}
type FISAUserInfo struct {
   Cn string `json:"cn"`
   ChName string `json:"chName"`
   Phone string `json:"phone"`
   Email string `json:"email"`
   InstCode string `json:"instCode"`
   Sysid string `json:"sysid"`
}

// 自定義的 ResponseWriter
type CustomResponseWriter struct {
   http.ResponseWriter
   RecordedHeaders http.Header
}

func(w *CustomResponseWriter) Header() http.Header {
	return w.RecordedHeaders
}

// Step 0. Url Fetch
func(app *Oauth2) UrlFetch(urlz string, params map[string]string)([]byte, error) {
   query := url.Values{}
   for key, value := range params {
      query.Add(key, value)
   }
	urlWithParams := fmt.Sprintf("%s?%s", urlz, query.Encode())
	response, err := http.Get(urlWithParams)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	// 讀取回應的內容
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// Step 1. 取得 Access Token
func(app *Oauth2) GetFISAAccessToken(code string)(*AccessToken, error) {
	params := map[string]string {
		"grant_type": "authorization_code",
		"client_id": app.ClientID,
		"client_secret": app.ClientSecret,
		"redirect_uri": app.RedirectUri,
		"code": code,
	}
	// 讀取回應的內容
	body, err := app.UrlFetch(app.TokenUrl, params)
	if err != nil {
		return nil, err
	}
	var accessToken AccessToken
	if err := json.Unmarshal(body, &accessToken); err != nil {
		return nil, err
	}
	return &accessToken, nil
}

// 取得個人資料 via accessToken
func(app *Oauth2) GetFISAUserInfo(accessToken string) (*FISAUserInfo, error) {
	params := map[string]string {
		"access_token": accessToken,
		// "fields": "id,name,email,gender,birthday,phone,address,postcode,city,country,avatar,created_at,updated_at",
	}
	// 讀取回應的內容
	body, err := app.UrlFetch(app.UserUrl, params)
	if err != nil {
		return nil, err
	}

	var userInfo FISAUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, err
	}
	return &userInfo, nil
}

// 取得個人資料 from web's code
func(app *Oauth2) FISAGetUserInfoViaCode(code string)(*FISAUserInfo, error) {
   accessToken, err := app.GetFISAAccessToken(code)   // 先取得 Access Token
   if err != nil {
      return nil, err
   }
   if accessToken.AccessToken == "" {
      return nil, fmt.Errorf("Error: Access Token is empty:" + code)
   }
   return app.GetFISAUserInfo(accessToken.AccessToken)
}

// 登出
func(app *Oauth2) Logout(w http.ResponseWriter, r *http.Request) {
   url := "/"
   session, err := app.Server.SessionManager.Get(r, "fisaOauth")
   if err != nil {
      http.Redirect(w, r, url, http.StatusTemporaryRedirect)
   }
   session.Options.MaxAge = -1
   delete(session.Values, "token")
   delete(session.Values, "email")
   w.Header().Del("Authorization")
   if err := session.Save(r, w); err!= nil {
      http.Redirect(w, r, url, http.StatusTemporaryRedirect)
   }
   http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// Protect 認證完成後，回到這個網址
func(app *Oauth2) FISAAuthenticate(w http.ResponseWriter, r *http.Request, code string)(http.ResponseWriter, error) {
   if code == "" {
      return w, fmt.Errorf("code is empty.")
   }
   userinfo, err := app.FISAGetUserInfoViaCode(code)  // 取得個人資料
   if err != nil {
      return w, fmt.Errorf("Get User info via code Error: %s", err.Error())
   }
   token := jwt.New(jwt.SigningMethodHS256)
   claims := token.Claims.(jwt.MapClaims)
   claims["exp"] = time.Now().Add(time.Hour * 24).Unix()
   claims["data"] = userinfo
   tokenString, err := token.SignedString([]byte(app.JwtKey)) // 簽名 JWT
   if err != nil {
      return w, fmt.Errorf("Sign JWT Error: %s", err.Error())
   }
   // 寫入 session
   session, err := app.Server.SessionManager.Get(r, "fisaOauth")
   if err != nil {
      return w, fmt.Errorf("Get Session Error: %s", err.Error())
   }
   session.Values["email"] = userinfo.Email   // 將Email存入Session
   session.Values["token"] = tokenString      // 將Token存入Session
   if err := session.Save(r, w); err!= nil {
      return w, fmt.Errorf("Save Session Error: %s", err.Error())
   }
   // 將 JWT 寫入 HTTP 標頭
   customWriter := &CustomResponseWriter{
      ResponseWriter: w, 
      RecordedHeaders: make(http.Header),
   }
   customWriter.Header().Set("Content-Type", "application/json; charset=utf-8")
   customWriter.Header().Set("Authorization", "Bearer " + tokenString)
   return customWriter, nil
}

// 未登入，轉到 FISA 認證
func(app *Oauth2) FISAAuthorize(w http.ResponseWriter, r *http.Request) {
   state, err := app.State(32)
   if err!= nil {
      http.Error(w, err.Error(), http.StatusInternalServerError)
      return
   }
   url := fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s", app.Endpoint, app.ClientID, app.RedirectUri, app.Scopes, state)
   http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}
