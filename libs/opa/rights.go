package SryOPA

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/open-policy-agent/opa/rego"
)

type Rights struct {
	User   string `json:"user"`
	Action string `json:"action"`
	Object string `json:"object"`
}
type WebQuery struct {
	SystemName string  `json:"systemName"`
	UserInfo   *Rights `json:"userInfo"`
}

func (app *SryOpa) ReadPolicy(fileName string) ([]byte, error) {
	if fileName == "" {
		return nil, fmt.Errorf("fileName is empty")
	}
	return os.ReadFile(fileName)
}
func (app *SryOpa) checkRights(system string, q *Rights) error {
	if system == "" || q == nil || q.User == "" || q.Action == "" || q.Object == "" {
		return fmt.Errorf("OPA check: system, user, action and object are required")
	}
	file := filepath.Join(app.Path, "rego", system+".rego")
	policy, err := app.ReadPolicy(file)
	if err != nil {
		return fmt.Errorf("policy file not found: %w", err)
	}
	query, err := rego.New(rego.Query("x = data.rbac.authz.allow"), rego.Module(file, string(policy))).PrepareForEval(context.Background())
	if err != nil {
		return err
	}
	result, err := query.Eval(context.Background(), rego.EvalInput(map[string]interface{}{"user": []string{q.User}, "action": q.Action, "object": q.Object}))
	if err != nil {
		return fmt.Errorf("evaluation error: %w", err)
	}
	if len(result) == 0 {
		return fmt.Errorf("undefined policy result")
	}
	allowed, ok := result[0].Bindings["x"].(bool)
	if !ok || !allowed {
		return fmt.Errorf("%s not allow %s %s", q.User, q.Action, q.Object)
	}
	return nil
}

func (app *SryOpa) CheckRightsFromMiddleWare(_ http.ResponseWriter, r *http.Request) error {
	token, err := bearer(r)
	if err != nil {
		return err
	}
	claims, err := app.parseToken(token)
	if err != nil {
		return err
	}
	system, _ := claims["iss"].(string)
	user, _ := claims["sub"].(string)
	return app.checkRights(system, &Rights{User: user, Action: r.Method, Object: r.URL.Path})
}

func (app *SryOpa) CheckRightsFromWeb(w http.ResponseWriter, r *http.Request) {
	if err := app.CheckRightsFromMiddleWare(w, r); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"allow"}`))
}
