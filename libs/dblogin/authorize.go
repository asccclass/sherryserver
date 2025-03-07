package DBLoginService

import(
   "fmt"
   "net/http"
)

func(app *DBLoginService) Authorize(r *http.Request)(error) {
   name := r.FormValue("username")
   user, ok := app.users[name]
   if !ok {
      return fmt.Errorf("Unauthorized")
   }

   // Get the session token from the cookie
   st, err := r.Cookie("session_token")
   if err != nil || st.Value == "" || st.Value != user.SessionToken {
      return fmt.Errorf("Unauthorized")
   }

   // get the CSRF token from the headers
   csrf := r.Header.Get("X-CSRF-Token")
   if csrf != user.CSRFToken || csrf == "" {
      return fmt.Errorf("Unauthorized")
   }

   return nil
}
