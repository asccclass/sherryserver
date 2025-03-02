package DBLoginService

import(
   "fmt"
   "time"
   "net/http"
)

func(app *DBLoginService) protected(w http.ResponseWriter, r *http.Request) {
   if r.Method != http.MethodPost {
      http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
      return
   }

   if err := app.Authorize(r); err != nil {
      http.Error(w, "Invalid method", http.StatusUnauthorized)
      return
   }
   name := r.FormValue("username")

   fmt.Fprintf(w, "CSRF validation successful! Welcome, %s", name)
}
