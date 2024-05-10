/*
   中央研究院 FISA SSO 服務介面
	// 
*/
package Oauth

import(
   // "os"
	// "io"
   "fmt"
	"net/url"
   "net/http"
	"io/ioutil"
	"encoding/json"
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

// 取得個人資料
func(app *Oauth2) GetFISAUserInfo(accessToken string) ([]byte, error) {
	params := map[string]string {
		"access_token": accessToken,
		// "fields": "id,name,email,gender,birthday,phone,address,postcode,city,country,avatar,created_at,updated_at",
	}
	// 讀取回應的內容
	body, err := app.UrlFetch(app.UserUrl, params)
	if err != nil {
		return nil, err
	}
/*	
	var accessToken AccessToken
	if err := json.Unmarshal(body, &accessToken); err != nil {
		return nil, err
	}
*/
	return body, nil
}

// 取得個人資料 from web's code
func(app *Oauth2) FISAGetUserInfoViaCode(code string)(error) {
   accessToken, err := app.GetFISAAccessToken(code)   // 先取得 Access Token
   if err != nil {
      return err
   }
	if accessToken.AccessToken == "" {
		return fmt.Errorf("Error: Access Token is empty")
	}
	res, err := app.GetFISAUserInfo(accessToken.AccessToken)
	if err != nil {
		return err
	}
   fmt.Println(string(res))
	return nil
}

// 認證完成後，回到這個網址
func(app *Oauth2) FISAAuthenticate(w http.ResponseWriter, r *http.Request, code string) {   
	// 取得個人資料
	err := app.FISAGetUserInfoViaCode(code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	/*
	t, err := conf.Exchange(context.Background(), code)
	client := conf.Client(context.Background(), t)

	// 取得使用者資訊
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
	}
	defer resp.Body.Close()
   var v any
	// Reading the JSON body using JSON decoder
	if err := json.NewDecoder(resp.Body).Decode(&v); err!= nil {
	   http.Error(w, err.Error(), http.StatusInternalServerError)
	   return
	}
	app.SessionManager.Put(r.Context(), "email", v.Email)  // 將Email存入Session
	http.Redirect(w, r, "/dashboard", http.StatusTemporaryRedirect)
	*/
}

// 轉到 FISA 認證
func(app *Oauth2) FISAAuthorize(w http.ResponseWriter, r *http.Request) {
	state, err := app.State(32)
	if err!= nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	url := fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s", app.Endpoint, app.ClientID, app.RedirectUri, app.Scopes, state)   
	fmt.Println("url: ", url)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}