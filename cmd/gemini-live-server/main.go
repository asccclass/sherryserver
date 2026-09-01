package main

import (
	"bufio"
	"mime"
	"net"
	"net/http"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
	"time"

	SherryServer "github.com/asccclass/sherryserver"
	"github.com/asccclass/sherryserver/internal/avatarconfig"
	"github.com/asccclass/sherryserver/internal/config"
	"github.com/asccclass/sherryserver/internal/llm"
	"github.com/asccclass/sherryserver/internal/orchestrator"
	"github.com/asccclass/sherryserver/internal/session"
	httptransport "github.com/asccclass/sherryserver/internal/transport/http"
	wstransport "github.com/asccclass/sherryserver/internal/transport/ws"
	"github.com/joho/godotenv"
)

func init() {
	_ = mime.AddExtensionType(".js", "text/javascript; charset=utf-8")
	_ = mime.AddExtensionType(".mjs", "text/javascript; charset=utf-8")
}

func main() {
	_ = godotenv.Load("envfile")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		panic(err)
	}
	avatarStore, err := avatarconfig.Load(filepath.Join(wdOrFallback(), "avatars.json"))
	if err != nil {
		panic(err)
	}

	server, err := SherryServer.NewServer(cfg.ListenAddr, cfg.Web.StaticPath, "")
	if err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	sessionManager := session.NewManager()
	interrupts := orchestrator.NewInterruptController()
	pageHandler := httptransport.NewPageHandler(httptransport.DefaultTemplatesRoot(wdOrFallback()), avatarStore, &cfg.MCP)
	liveHandler := wstransport.NewLiveHandler(wstransport.HandlerConfig{
		GeminiAPIKey:      cfg.Gemini.APIKey,
		GeminiAPIVersion:  cfg.Gemini.APIVersion,
		ProcessingMessage: cfg.LLM.ProcessingMessage,
		ErrorMessage:      cfg.LLM.ErrorMessage,
		Sessions:          sessionManager,
		Interrupts:        interrupts,
		LLM: llm.OpenAICompatConfig{
			APIURL:       cfg.LLM.APIURL,
			APIKey:       cfg.LLM.APIKey,
			Model:        cfg.LLM.Model,
			SystemPrompt: cfg.LLM.SystemPrompt,
			Timeout:      cfg.LLM.Timeout,
		},
		MCP:         &cfg.MCP,
		AvatarStore: avatarStore,
	})
	sessionAPI := httptransport.NewSessionAPI(sessionManager, interrupts)
	avatarAPI := httptransport.NewAvatarAPI(avatarStore)
	pageHandler.Register(mux)
	mux.Handle("GET /ws/live", liveHandler)
	mux.HandleFunc("GET /healthz", httptransport.Health)
	mux.HandleFunc("GET /api/avatars", avatarAPI.ListAvatars)
	mux.HandleFunc("POST /api/session/create", sessionAPI.CreateSession)
	mux.HandleFunc("GET /api/session/{id}", sessionAPI.GetSession)
	mux.HandleFunc("GET /api/session/{id}/events", sessionAPI.SessionEvents)
	mux.HandleFunc("POST /api/session/{id}/interrupt", sessionAPI.InterruptSession)
	registerStaticRoutes(mux, cfg.Web.StaticPath)

	server.Server.Handler = withStaticAssetFallback(mux, cfg.Web.StaticPath)
	logMCPHealth(server, cfg.MCP.SSEURL)
	server.Start()
}

func wdOrFallback() string {
	wd, err := os.Getwd()
	if err == nil {
		return wd
	}
	return "."
}

func registerStaticRoutes(mux *http.ServeMux, staticRoot string) {
	mux.Handle("GET /avatar/", http.StripPrefix("/avatar/", http.FileServer(http.Dir(filepath.Join(staticRoot, "avatar")))))
	mux.Handle("GET /headaudio/", http.StripPrefix("/headaudio/", http.FileServer(http.Dir(filepath.Join(staticRoot, "headaudio")))))
	mux.Handle("GET /vendor/", http.StripPrefix("/vendor/", http.FileServer(http.Dir(filepath.Join(staticRoot, "vendor")))))
	mux.Handle("GET /web/", http.StripPrefix("/web/", http.FileServer(http.Dir(filepath.Join(staticRoot, "web")))))
}

func withStaticAssetFallback(next http.Handler, staticRoot string) http.Handler {
	staticFallback := serveStaticAsset(staticRoot)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(recorder, r)
		if recorder.wroteHeader && recorder.statusCode != http.StatusNotFound {
			return
		}
		if recorder.wroteBody && recorder.statusCode != http.StatusNotFound {
			return
		}

		staticFallback.ServeHTTP(w, r)
	})
}

func serveStaticAsset(staticRoot string) http.Handler {
	staticRoot = filepath.Clean(staticRoot)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}

		relativePath := strings.TrimPrefix(pathpkg.Clean(r.URL.Path), "/")
		if relativePath == "." || relativePath == "" {
			http.NotFound(w, r)
			return
		}

		fullPath := filepath.Join(staticRoot, relativePath)
		relToRoot, err := filepath.Rel(staticRoot, fullPath)
		if err != nil || strings.HasPrefix(relToRoot, "..") {
			http.NotFound(w, r)
			return
		}

		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}

		if contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(fullPath))); contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		http.ServeFile(w, r, fullPath)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
	wroteBody   bool
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.wroteHeader = true
	if statusCode != http.StatusNotFound {
		r.ResponseWriter.WriteHeader(statusCode)
	}
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	r.wroteBody = true
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	if r.statusCode == http.StatusNotFound {
		return len(body), nil
	}
	return r.ResponseWriter.Write(body)
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func logMCPHealth(server *SherryServer.Server, endpoint string) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		server.Logger.Warn("MCP health check skipped: MCP_SSE_URL is empty")
		return
	}

	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		server.Logger.Warn("MCP health check request build failed")
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		server.Logger.Warn("MCP health check failed: " + err.Error() + " endpoint=" + endpoint)
		return
	}
	defer resp.Body.Close()

	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if resp.StatusCode == http.StatusOK && strings.Contains(contentType, "text/event-stream") {
		server.Logger.Info("MCP health check ok: endpoint=" + endpoint + " content-type=" + contentType)
		return
	}

	server.Logger.Warn("MCP health check unexpected response: endpoint=" + endpoint + " status=" + resp.Status + " content-type=" + contentType)
}
