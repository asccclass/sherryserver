package DBLoginService

import(
   "fmt"
   "net/http"
)

/*
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
      code := r.URL.Query().Get("code")  
      if !ok || email == "" {  
         if code == "" {
            app.FISAAuthorize(w, r)    // 未登入，導向登入頁面
            return
         } else {
            // fmt.Println("登入成功，導向原本頁面")
            var err error
            w, err = app.FISAAuthenticate(w, r, code) // 登入成功，導向原本頁面
            if err != nil { // 登入成功，導向原本頁面
               fmt.Println("登入成功，但 FISAAuthenticate 失敗:", err.Error())
               return
            }
            next.ServeHTTP(w, r) 
         }
      } else {  // 登入過
         tokenString, ok := session.Values["token"].(string)   
         if !ok {
            fmt.Println("JWT 失效，導向登入頁面", err.Error())
            app.FISAAuthorize(w, r)    // JWT 失效，導向登入頁面
            return
         }
         // 將 JWT 寫入 HTTP 標頭
         customWriter := &CustomResponseWriter{
            ResponseWriter: w, 
            RecordedHeaders: make(http.Header),
         }
         customWriter.Header().Set("Content-Type", "application/json; charset=utf-8")
         customWriter.Header().Set("Authorization", "Bearer " + tokenString)
         // fmt.Println(email + "已經登入，進入Home Page頁面")
         next.ServeHTTP(customWriter, r)
      }
	})
}
*/

func(app *DBLoginService) register(w http.ResponseWriter, r *http.Request) {
   if r.Method != http.MethodPost {
      http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
      return
   }
   name := r.FormValue("username")
   pass := r.FormValue("password")
   if len(name) < 8 || len(pass) < 8 {
      http.Error(w, "Invalid user name or password", http.StatusNotAcceptable)
      return
   }
   if _, ok := app.users[name]; ok {
      http.Error(w, "User already exists", http.StatusConflict)
      return
   }

   hashPassword, _ := app.hashPassword(pass)
   app.users[name] = Login {
      HashedPassword: hashPassword,
   }
   fmt.Fprintln(w, "User registered successfully!")
}
