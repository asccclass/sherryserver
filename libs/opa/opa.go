package SryOPA

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"

	SherryServer "github.com/asccclass/sherryserver"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
)

type SryOpa struct {
	Srv     *SherryServer.Server
	Path    string
	OpaFile string
	Secret  string
}

func NewOpa(srv *SherryServer.Server, path string) (*SryOpa, error) {
	if srv == nil {
		return nil, fmt.Errorf("server is nil")
	}
	if path == "" {
		return nil, fmt.Errorf("pool path is empty")
	}
	opafile := os.Getenv("OPAFile")
	if opafile == "" {
		return nil, fmt.Errorf("OPAFile is not set in envfile")
	}
	secret := os.Getenv("OPAJWTSecret")
	if secret == "" {
		return nil, fmt.Errorf("OPA JWT secret is not set in OPAJWTSecret")
	}
	return &SryOpa{Srv: srv, Path: path, OpaFile: opafile, Secret: secret}, nil
}

func (app *SryOpa) AddRouter(router *mux.Router) {
	router.HandleFunc("/accesstoken", app.getAccessTokenFromWeb).Methods(http.MethodGet)
	router.HandleFunc("/authorize", app.CheckRightsFromWeb).Methods(http.MethodPost)
}

func (app *SryOpa) newToken(system, user string) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"iss": system, "sub": user}).SignedString([]byte(app.Secret))
}

func (app *SryOpa) parseToken(token string) (jwt.MapClaims, error) {
	t, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(app.Secret), nil
	})
	if err != nil || !t.Valid {
		if err == nil {
			err = fmt.Errorf("invalid JWT token")
		}
		return nil, err
	}
	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid JWT claims")
	}
	return claims, nil
}

func bearer(r *http.Request) (string, error) {
	v := strings.TrimSpace(r.Header.Get("Authorization"))
	if v == "" {
		return "", fmt.Errorf("Authorization header is required")
	}
	p := strings.Fields(v)
	if len(p) != 2 || !strings.EqualFold(p[0], "Bearer") {
		return "", fmt.Errorf("invalid Authorization header")
	}
	return p[1], nil
}

func randomClientToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
