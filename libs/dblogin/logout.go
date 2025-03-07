package DBLoginService

import(
   "fmt"
   "time"
   "net/http"
)

func(app *DBLoginService) logout(w http.ResponseWriter, r *http.Request) {
   if err := app.Authorize(r); err != nil {
      http.Error(w, "Unauthorized", http.StatusUnauthorized)
      return
   }

   // clear cookie
   http.SetCookie(w, &http.Cookie {
      Name:  "session_token",
      Value: "",
      Expires: time.Now().Add(-time.Hour),
      HttpOnly: true,
   })
   http.SetCookie(w, &http.Cookie {
      Name:  "csrf_token",
      Value: "",
      Expires: time.Now().Add(-time.Hour),
      HttpOnly: false,
   })

   // clear the tokens from the database
   name := r.FormValue("username")
   user, _ := app.users[name]
   user.SessionToken = ""
   user.CSRFToken = ""
   app.users[name] = user

   fmt.Fprintf(w, "%s logged out successful!", name)
}
