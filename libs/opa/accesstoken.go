package SryOPA

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type ApplicationRecord struct {
	ClientID     string `json:"appName"`
	User         string `json:"clientID"`
	ClientSecret string `json:"clientSecret"`
	Deadline     string `json:"deadline"`
}
type ApplicationRecords struct {
	Apps []ApplicationRecord `json:"Applications"`
}

func (app *SryOpa) getAccessTokenFromWeb(w http.ResponseWriter, r *http.Request) {
	clientToken, err := bearer(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	claims, err := app.parseToken(clientToken)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	content, err := app.ReadPolicy(app.OpaFile)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var apps ApplicationRecords
	if err = json.Unmarshal(content, &apps); err != nil {
		http.Error(w, fmt.Sprintf("app unmarshal error: %v", err), http.StatusInternalServerError)
		return
	}
	system, _ := claims["iss"].(string)
	user, _ := claims["sub"].(string)
	allowed := false
	for _, a := range apps.Apps {
		if a.ClientID == system && a.User == user {
			allowed = true
			break
		}
	}
	if !allowed {
		http.Error(w, "clientID not found", http.StatusUnauthorized)
		return
	}
	token, err := app.newToken(system, user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	json.NewEncoder(w).Encode(map[string]string{"accessToken": token})
}
