/*
   中央研究院 FISA SSO 服務介面
	// 
*/
package Oauth

import(
   // "os"
	// "io"
   "fmt"
   "net/http"
)


// 認證完成後，回到這個網址
func(app *Oauth2) FISAAuthenticate(w http.ResponseWriter, r *http.Request, code) {
   code := r.URL.Query().Get("code")
	/*
	t, err := conf.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
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
	fmt.Println("code: ", code)
}

// 轉到 FISA 認證
func(app *Oauth2) FISAAuthorize(w http.ResponseWriter, r *http.Request) {
	state, err := app.State(32)
	if err!= nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	url := fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s", app.Endpoint, app.ClientID, app.RedirectUri, app.Scopes, state)   
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}