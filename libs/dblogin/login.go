package DBLoginService

import(
   "fmt"
   "time"
   "net/http"
)

func(app *DBLoginService) login(w http.ResponseWriter, r *http.Request) {
   if r.Method != http.MethodPost {
      http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
      return
   }
   name := r.FormValue("username")
   pass := r.FormValue("password")

   user, ok := app.users[name]
   if !ok || !app.checkPasswordHash(pass, user.HashedPassword) {
      http.Error(w, "Invalid username or password", http.StatusUnauthorized)
      return
   }

   sessionToken := app.generateToken(32)
   csrfToken := app.generateToken(32)

   http.SetCookie(w, &http.Cookie {
      Name:  "session_token",
      Value: sessionToken,
      Expires:  time.Now().Add(24 * time.Hour),
      HttpOnly: true,
   })

   // set CSRF token in a cookeie
   http.SetCookie(w, &http.Cookie {
      Name: "csrf_token",
      Value: csrfToken,
      Expires: time.Now().Add(24 * time.Hour),
      HttpOnly: false,  // Needs to be accessible to the client-side
   })

   // store tokens in the database
   user.SessionToken = sessionToken
   user.CSRFToken = csrfToken
   app.users[name] = user

   fmt.Fprintln(w, "Login successful!")
}
