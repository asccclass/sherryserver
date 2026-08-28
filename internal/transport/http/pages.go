package httptransport

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/asccclass/sherryserver/internal/avatarconfig"
	"github.com/asccclass/sherryserver/internal/config"
	"github.com/asccclass/sherryserver/internal/llm"
)

type HomeAgent struct {
	ID          string
	Emoji       string
	Name        string
	Title       string
	Description string
	AvatarClass string
	ButtonClass string
	AccentColor string
	Persona     string
	Featured    bool
	Route       string
}

type PageHandler struct {
	templatesRoot string
	avatarStore   *avatarconfig.Store
	mcpConfig     *config.MCPConfig
	homeAgents    []HomeAgent
}

type HomePageData struct {
	Agents []HomeAgent
}

type FridayPageData struct {
	Avatar                avatarconfig.Avatar
	TemperatureValue      string
	InputTranscriptionOn  bool
	OutputTranscriptionOn bool
	MCPEnabled            bool
	MCPEndpoint           string
}

type FridayMCPResultData struct {
	Query     string
	Result    string
	Error     string
	ToolName  string
	Endpoint  string
	QueriedAt string
	IsEmpty   bool
	HasResult bool
}

func NewPageHandler(templatesRoot string, avatarStore *avatarconfig.Store, mcpConfig *config.MCPConfig) *PageHandler {
	return &PageHandler{
		templatesRoot: templatesRoot,
		avatarStore:   avatarStore,
		mcpConfig:     mcpConfig,
		homeAgents:    defaultHomeAgents(),
	}
}

func (h *PageHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", h.Home)
	mux.HandleFunc("GET /avatar/friday", h.Friday)
	mux.HandleFunc("GET /friday.html", h.FridayRedirect)
	mux.HandleFunc("GET /friday/mcp", h.FridayMCPPanel)
	mux.HandleFunc("GET /friday/mcp/query", h.FridayMCPQuery)
}

func (h *PageHandler) Home(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, http.StatusOK, "home.gohtml", HomePageData{
		Agents: h.homeAgents,
	})
}

func (h *PageHandler) Friday(w http.ResponseWriter, r *http.Request) {
	avatar, ok := h.avatarStore.Get("friday")
	if !ok {
		http.Error(w, "friday avatar not found", http.StatusNotFound)
		return
	}

	h.renderTemplate(w, http.StatusOK, "friday.gohtml", FridayPageData{
		Avatar:                avatar,
		TemperatureValue:      float32String(avatar.Temperature, "0.8"),
		InputTranscriptionOn:  boolValue(avatar.EnableInputTranscription, true),
		OutputTranscriptionOn: boolValue(avatar.EnableOutputTranscription, true),
		MCPEnabled:            strings.TrimSpace(h.resolveFridayEndpoint()) != "",
		MCPEndpoint:           h.resolveFridayEndpoint(),
	})
}

func (h *PageHandler) FridayRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/avatar/friday", http.StatusTemporaryRedirect)
}

func (h *PageHandler) FridayMCPPanel(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, http.StatusOK, "friday_mcp_result.gohtml", FridayMCPResultData{
		Endpoint:  h.resolveFridayEndpoint(),
		ToolName:  h.resolveFridayToolName(),
		QueriedAt: time.Now().Format("2006-01-02 15:04:05"),
		IsEmpty:   true,
	})
}

func (h *PageHandler) FridayMCPQuery(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	data := FridayMCPResultData{
		Query:     query,
		Endpoint:  h.resolveFridayEndpoint(),
		ToolName:  h.resolveFridayToolName(),
		QueriedAt: time.Now().Format("2006-01-02 15:04:05"),
		IsEmpty:   query == "",
	}

	if strings.TrimSpace(data.Endpoint) == "" {
		data.Error = "MCP_SSE_URL 尚未設定，無法查詢。"
		h.renderTemplate(w, http.StatusOK, "friday_mcp_result.gohtml", data)
		return
	}

	if data.IsEmpty {
		h.renderTemplate(w, http.StatusOK, "friday_mcp_result.gohtml", data)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	reply, err := llm.NewMCPQueryClient(llm.MCPQueryConfig{
		Endpoint:     data.Endpoint,
		ToolName:     data.ToolName,
		ToolArgument: h.resolveFridayToolArgument(),
	}).Generate(ctx, llm.Prompt{User: query})
	if err != nil {
		data.Error = err.Error()
		h.renderTemplate(w, http.StatusOK, "friday_mcp_result.gohtml", data)
		return
	}

	data.Result = strings.TrimSpace(reply.Text)
	data.HasResult = data.Result != ""
	if !data.HasResult {
		data.Error = "MCP 沒有回傳可顯示的內容。"
	}

	h.renderTemplate(w, http.StatusOK, "friday_mcp_result.gohtml", data)
}

func (h *PageHandler) renderTemplate(w http.ResponseWriter, status int, name string, data any) {
	tpl, err := template.ParseFiles(filepath.Join(h.templatesRoot, name))
	if err != nil {
		http.Error(w, fmt.Sprintf("template parse error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := tpl.Execute(w, data); err != nil {
		http.Error(w, fmt.Sprintf("template render error: %v", err), http.StatusInternalServerError)
	}
}

func DefaultTemplatesRoot(wd string) string {
	configured := strings.TrimSpace(os.Getenv("TemplateRoot"))
	if configured != "" {
		if filepath.IsAbs(configured) {
			return configured
		}
		return filepath.Join(wd, configured)
	}
	return filepath.Join(wd, "www", "template")
}

func defaultHomeAgents() []HomeAgent {
	return []HomeAgent{
		{ID: "ceo", Emoji: "🧠", Name: "CEO", Title: "執行長", Description: "統籌規劃、拆解任務、協調各 Agent", AvatarClass: "av-ceo", ButtonClass: "cb-ceo", AccentColor: "#534AB7", Route: "/avatar/friday?agent=ceo", Persona: "# 角色：CEO（執行長）"},
		{ID: "writer", Emoji: "✍️", Name: "寫手", Title: "程式設計師", Description: "執行任務書、撰寫代碼、自我初審後提交", AvatarClass: "av-writer", ButtonClass: "cb-writer", AccentColor: "#0F6E56", Route: "/avatar/friday?agent=writer", Persona: "# 角色：寫手（程式設計師）"},
		{ID: "reviewer", Emoji: "🔍", Name: "找查", Title: "程式審查員", Description: "全面審查代碼品質，有問題退回寫手", AvatarClass: "av-reviewer", ButtonClass: "cb-reviewer", AccentColor: "#185FA5", Route: "/avatar/friday?agent=reviewer", Persona: "# 角色：找查（程式審查員）"},
		{ID: "security", Emoji: "🛡️", Name: "吱吱", Title: "資安健檢師", Description: "白帽滲透測試（Strix），資安最後一關", AvatarClass: "av-security", ButtonClass: "cb-security", AccentColor: "#993C1D", Route: "/avatar/friday?agent=security", Persona: "# 角色：吱吱（資安健檢師）"},
		{ID: "docs", Emoji: "📁", Name: "檔管", Title: "文檔管理者", Description: "統一歸檔、索引、即時查詢所有工作文件", AvatarClass: "av-docs", ButtonClass: "cb-docs", AccentColor: "#854F0B", Featured: true, Route: "/avatar/friday", Persona: "# 角色：檔管（文檔管理者）"},
	}
}

func float32String(value *float32, fallback string) string {
	if value == nil {
		return fallback
	}
	return fmt.Sprintf("%.1f", *value)
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func (h *PageHandler) mcpSSEURL() string {
	if h.mcpConfig == nil {
		return ""
	}
	return strings.TrimSpace(h.mcpConfig.SSEURL)
}

func (h *PageHandler) resolveFridayEndpoint() string {
	if strings.TrimSpace(h.mcpSSEURL()) != "" {
		return strings.TrimSpace(h.mcpSSEURL())
	}
	avatar, ok := h.avatarStore.Get("friday")
	if ok && strings.TrimSpace(avatar.Endpoint) != "" {
		return strings.TrimSpace(avatar.Endpoint)
	}
	return ""
}

func (h *PageHandler) resolveFridayToolName() string {
	avatar, ok := h.avatarStore.Get("friday")
	if ok {
		return strings.TrimSpace(avatar.MCPToolName)
	}
	return ""
}

func (h *PageHandler) resolveFridayToolArgument() string {
	avatar, ok := h.avatarStore.Get("friday")
	if ok {
		return strings.TrimSpace(avatar.MCPToolArgument)
	}
	return ""
}
