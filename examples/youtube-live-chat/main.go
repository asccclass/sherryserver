package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	SherryServer "github.com/asccclass/sherryserver"
	"github.com/asccclass/sherryserver/examples/youtube-live-chat/internal/model"
	"github.com/asccclass/sherryserver/examples/youtube-live-chat/internal/splitter"
	"github.com/asccclass/sherryserver/examples/youtube-live-chat/internal/youtube"
	"github.com/coder/websocket"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

const (
	defaultPollInterval = 2 * time.Second
)

type startRequest struct {
	VideoID string `json:"videoId"`
	APIKey  string `json:"apiKey"`
}

type uiEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data,omitempty"`
	Err  string      `json:"error,omitempty"`
	At   string      `json:"at"`
}

type uiHub struct {
	logger  *zap.Logger
	mu      sync.RWMutex
	clients map[*websocket.Conn]*sync.Mutex
}

func newUIHub(logger *zap.Logger) *uiHub {
	return &uiHub{
		logger:  logger,
		clients: make(map[*websocket.Conn]*sync.Mutex),
	}
}

func (h *uiHub) add(conn *websocket.Conn) {
	h.mu.Lock()
	h.clients[conn] = &sync.Mutex{}
	h.mu.Unlock()
}

func (h *uiHub) remove(conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
	conn.CloseNow()
}

func (h *uiHub) broadcast(event uiEvent) {
	event.At = time.Now().UTC().Format(time.RFC3339)
	payload, err := json.Marshal(event)
	if err != nil {
		h.logger.Error("marshal ui event failed", zap.Error(err))
		return
	}

	h.mu.RLock()
	clients := make([]struct {
		conn *websocket.Conn
		mu   *sync.Mutex
	}, 0, len(h.clients))
	for conn, mu := range h.clients {
		clients = append(clients, struct {
			conn *websocket.Conn
			mu   *sync.Mutex
		}{conn: conn, mu: mu})
	}
	h.mu.RUnlock()

	for _, c := range clients {
		c.mu.Lock()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := c.conn.Write(ctx, websocket.MessageText, payload)
		cancel()
		c.mu.Unlock()
		if err != nil {
			h.logger.Warn("write to ui websocket failed", zap.Error(err))
			h.remove(c.conn)
		}
	}
}

type app struct {
	logger        *zap.Logger
	hub           *uiHub
	yt            *youtube.Client
	defaultAPIKey string
	defaultWSURL  string
	defaultVideo  string
	allowedHosts  map[string]struct{}

	mu  sync.Mutex
	job *liveChatJob
}

func newApp(logger *zap.Logger) *app {
	defaultWSURL := os.Getenv("EXTERNAL_WS_URL")
	allowHosts := parseAllowedHosts(os.Getenv("EXTERNAL_WS_ALLOWED_HOSTS"))
	if len(allowHosts) == 0 && defaultWSURL != "" {
		if u, err := url.Parse(defaultWSURL); err == nil {
			host := strings.ToLower(u.Hostname())
			if host != "" {
				allowHosts[host] = struct{}{}
			}
		}
	}

	return &app{
		logger:        logger,
		hub:           newUIHub(logger),
		yt:            youtube.NewClient(nil),
		defaultAPIKey: os.Getenv("YOUTUBE_API_KEY"),
		defaultWSURL:  defaultWSURL,
		defaultVideo:  os.Getenv("YOUTUBE_VIDEO_ID"),
		allowedHosts:  allowHosts,
	}
}

func (a *app) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		a.logger.Error("accept ui websocket failed", zap.Error(err))
		return
	}
	a.hub.add(conn)
	defer a.hub.remove(conn)
	ctx := conn.CloseRead(r.Context())
	<-ctx.Done()
}

func (a *app) handleConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"videoId":       a.defaultVideo,
		"apiKey":        "",
		"externalWsUrl": a.defaultWSURL,
	})
}

func (a *app) handleStart(w http.ResponseWriter, r *http.Request) {
	var req startRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "request body 格式錯誤")
		return
	}
	req.VideoID = firstNonEmpty(req.VideoID, a.defaultVideo)
	req.APIKey = firstNonEmpty(req.APIKey, a.defaultAPIKey)
	externalWSURL := a.defaultWSURL

	if req.VideoID == "" || req.APIKey == "" || externalWSURL == "" {
		writeError(w, http.StatusBadRequest, "videoId、apiKey 與環境變數 EXTERNAL_WS_URL 都必須提供")
		return
	}
	if err := validateExternalWSURL(externalWSURL, a.allowedHosts); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := a.startJob(req, externalWSURL); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

func (a *app) handleStop(w http.ResponseWriter, _ *http.Request) {
	a.stopJob("使用者停止")
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (a *app) startJob(req startRequest, externalWSURL string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.job != nil {
		a.job.stop("切換到新的直播工作")
	}

	ctx, cancel := context.WithCancel(context.Background())
	job := &liveChatJob{
		app:           a,
		ctx:           ctx,
		cancel:        cancel,
		videoID:       req.VideoID,
		apiKey:        req.APIKey,
		externalWSURL: externalWSURL,
	}
	a.job = job
	a.hub.broadcast(uiEvent{Type: "status", Data: map[string]string{"message": "直播工作已啟動", "videoId": req.VideoID}})
	go job.run()
	return nil
}

func (a *app) stopJob(reason string) {
	a.mu.Lock()
	job := a.job
	a.mu.Unlock()
	if job != nil {
		job.stop(reason)
	}
}

func (a *app) clearJob(job *liveChatJob) {
	a.mu.Lock()
	if a.job == job {
		a.job = nil
	}
	a.mu.Unlock()
}

type liveChatJob struct {
	app           *app
	ctx           context.Context
	cancel        context.CancelFunc
	videoID       string
	apiKey        string
	externalWSURL string

	mu         sync.Mutex
	externalWS *websocket.Conn
	stopOnce   sync.Once
}

func (j *liveChatJob) stop(reason string) {
	j.stopOnce.Do(func() {
		j.app.hub.broadcast(uiEvent{Type: "status", Data: map[string]string{"message": reason}})
		j.cancel()
		j.mu.Lock()
		if j.externalWS != nil {
			j.externalWS.CloseNow()
		}
		j.mu.Unlock()
	})
}

func (j *liveChatJob) run() {
	defer j.app.clearJob(j)

	chatID, err := j.app.yt.ResolveActiveLiveChatID(j.ctx, j.apiKey, j.videoID)
	if err != nil {
		j.fail(fmt.Errorf("解析 activeLiveChatId 失敗: %w", err))
		return
	}

	conn, _, err := websocket.Dial(j.ctx, j.externalWSURL, nil)
	if err != nil {
		j.fail(fmt.Errorf("連線外部 WebSocket 失敗: %w", err))
		return
	}
	j.mu.Lock()
	j.externalWS = conn
	j.mu.Unlock()
	j.app.hub.broadcast(uiEvent{Type: "status", Data: map[string]string{"message": "已連線外部 WebSocket"}})

	go j.readExternalLoop(conn)
	j.pollLoop(chatID)
}

func (j *liveChatJob) readExternalLoop(conn *websocket.Conn) {
	for {
		_, data, err := conn.Read(j.ctx)
		if err != nil {
			if j.ctx.Err() != nil {
				return
			}
			j.fail(fmt.Errorf("接收外部 WebSocket 訊息失敗: %w", err))
			return
		}

		if json.Valid(data) {
			j.app.hub.broadcast(uiEvent{Type: "external", Data: json.RawMessage(data)})
		} else {
			j.app.hub.broadcast(uiEvent{Type: "external", Data: map[string]string{"message": string(data)}})
		}
	}
}

func (j *liveChatJob) pollLoop(chatID string) {
	defer j.stop("直播工作結束")

	pageToken := ""
	for {
		select {
		case <-j.ctx.Done():
			return
		default:
		}

		page, err := j.app.yt.FetchLiveChatMessages(j.ctx, j.apiKey, chatID, pageToken)
		if err != nil {
			if youtube.IsLiveEnded(err) {
				j.app.hub.broadcast(uiEvent{Type: "status", Data: map[string]string{"message": "直播已結束或聊天室已關閉"}})
				return
			}
			j.fail(fmt.Errorf("輪詢直播留言失敗: %w", err))
			return
		}
		pageToken = page.NextPageToken

		for _, m := range page.Messages {
			sentences := splitter.SplitSentences(m.Text)
			for _, sentence := range sentences {
				payload := model.LiveSentenceMessage{
					Text:        sentence,
					Author:      m.Author,
					VideoID:     j.videoID,
					MessageID:   m.ID,
					PublishedAt: m.PublishedAt.Format(time.RFC3339),
				}
				if err := j.sendSentence(payload); err != nil {
					j.fail(fmt.Errorf("送出訊息到外部 WebSocket 失敗: %w", err))
					return
				}
				j.app.hub.broadcast(uiEvent{Type: "sent", Data: payload})
			}
		}

		interval := page.PollingInterval
		if interval <= 0 {
			interval = defaultPollInterval
		}
		select {
		case <-j.ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

func (j *liveChatJob) sendSentence(payload model.LiveSentenceMessage) error {
	bytes, err := payload.Encode()
	if err != nil {
		return err
	}

	j.mu.Lock()
	conn := j.externalWS
	j.mu.Unlock()
	if conn == nil {
		return errors.New("外部 WebSocket 尚未建立")
	}

	ctx, cancel := context.WithTimeout(j.ctx, 5*time.Second)
	defer cancel()
	return conn.Write(ctx, websocket.MessageText, bytes)
}

func (j *liveChatJob) fail(err error) {
	j.app.logger.Error(err.Error())
	j.app.hub.broadcast(uiEvent{Type: "error", Err: err.Error()})
	j.stop("直播工作停止")
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

func parseAllowedHosts(raw string) map[string]struct{} {
	hosts := map[string]struct{}{}
	for _, item := range strings.Split(raw, ",") {
		host := strings.ToLower(strings.TrimSpace(item))
		if host == "" {
			continue
		}
		hosts[host] = struct{}{}
	}
	return hosts
}

func validateExternalWSURL(raw string, allowedHosts map[string]struct{}) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("externalWsUrl 格式不正確: %w", err)
	}
	if u.Scheme != "ws" && u.Scheme != "wss" {
		return errors.New("externalWsUrl 只允許 ws 或 wss")
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return errors.New("externalWsUrl 缺少 host")
	}
	if len(allowedHosts) == 0 {
		return errors.New("未設定可允許的 EXTERNAL_WS_ALLOWED_HOSTS")
	}
	if _, ok := allowedHosts[host]; !ok {
		return fmt.Errorf("externalWsUrl host %q 不在允許清單", host)
	}
	return nil
}

func main() {
	_ = godotenv.Load("envfile")

	port := firstNonEmpty(os.Getenv("PORT"), "8080")
	documentRoot := firstNonEmpty(os.Getenv("DocumentRoot"), "www/html")
	templateRoot := firstNonEmpty(os.Getenv("TemplateRoot"), "www/template")

	cwd, err := os.Getwd()
	if err == nil && !filepath.IsAbs(documentRoot) {
		documentRoot = filepath.Join(cwd, documentRoot)
	}

	srv, err := SherryServer.NewServer(":"+port, documentRoot, templateRoot)
	if err != nil {
		panic(err)
	}

	router := http.NewServeMux()
	application := newApp(srv.Logger)
	router.HandleFunc("GET /api/config", application.handleConfig)
	router.HandleFunc("POST /api/start", application.handleStart)
	router.HandleFunc("POST /api/stop", application.handleStop)
	router.HandleFunc("/ws", application.handleWS)

	static := SherryServer.StaticFileServer{StaticPath: documentRoot}
	static.AddRouter(router)

	srv.Server.Handler = router
	srv.Start()
}
