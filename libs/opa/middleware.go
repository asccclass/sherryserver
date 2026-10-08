package SryOPA

import (
	"encoding/json"
	"net/http"
)

func (app *SryOpa) Authorize(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := app.CheckRightsFromMiddleWare(w, r); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"errMsg": err.Error()})
			return
		}
		next(w, r)
	}
}
