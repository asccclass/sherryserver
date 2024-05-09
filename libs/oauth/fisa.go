/*
   中央研究院 FISA SSO 服務介面
	// 
*/
package Oauth

import(
   "os"
	"io"
   "strings"
   "net/http"
)


// 認證完成後，回到這個網址
func(app *Oauth2) FOSACallback(w http.ResponseWriter, r *http.Request) {
   code := r.URL.Query().Get("code")
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
	app.SessionManager.Put(r.Context(), "email", v.Email)
	http.Redirect(w, r, "/dashboard", http.StatusTemporaryRedirect)
}

// 要組合網址
func(app *Oauth2) FOSALogin(w http.ResponseWriter, r *http.Request) {
   url := conf.AuthCodeURL("state", oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
   // url := Oauth2.AuthCodeURL("state", oauth2.AccessTypeOffline)
   // http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}