package ws

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/asccclass/sherryserver/internal/avatarconfig"
	"github.com/asccclass/sherryserver/internal/config"
	"github.com/asccclass/sherryserver/internal/llm"
	"github.com/asccclass/sherryserver/internal/mcp"
	"github.com/asccclass/sherryserver/internal/orchestrator"
	"github.com/asccclass/sherryserver/internal/session"
	"github.com/asccclass/sherryserver/internal/stt"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"google.golang.org/genai"
)

type HandlerConfig struct {
	GeminiAPIKey      string
	GeminiAPIVersion  string
	LLM               llm.OpenAICompatConfig
	ProcessingMessage string
	ErrorMessage      string
	Sessions          *session.Manager
	Interrupts        *orchestrator.InterruptController
	MCP               *config.MCPConfig
	AvatarStore       *avatarconfig.Store
}

type LiveHandler struct {
	cfg      HandlerConfig
	upgrader websocket.Upgrader
}

type clientConnWriter struct {
	ws      *websocket.Conn
	writeMu *sync.Mutex
}

func NewLiveHandler(cfg HandlerConfig) *LiveHandler {
	return &LiveHandler{
		cfg: cfg,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (w *clientConnWriter) SendStatus(message string) {
	if strings.TrimSpace(message) == "" {
		return
	}
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	_ = w.ws.WriteJSON(ServerEnvelope{Type: "status", Message: message})
}

func (w *clientConnWriter) SendPrompt(systemPrompt string, userPrompt string) {
	if strings.TrimSpace(systemPrompt) == "" && strings.TrimSpace(userPrompt) == "" {
		return
	}
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	_ = w.ws.WriteJSON(ServerEnvelope{
		Type: "llm_prompt",
		Message: map[string]string{
			"systemPrompt": systemPrompt,
			"userPrompt":   userPrompt,
		},
	})
}

func (w *clientConnWriter) SendAssistantText(text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	_ = w.ws.WriteJSON(ServerEnvelope{
		Type: "assistant_text",
		Message: map[string]string{
			"text": text,
		},
	})
}

func (h *LiveHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	clientWS, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer clientWS.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if sessionID == "" {
		sessionID = strings.TrimSpace(r.URL.Query().Get("sessionId"))
	}
	sessionIDFromQuery := sessionID != ""
	if sessionID == "" {
		sessionID = uuid.NewString()
	}

	if h.cfg.Sessions != nil {
		if _, ok := h.cfg.Sessions.Get(sessionID); !ok {
			h.cfg.Sessions.Upsert(&session.Session{
				ID:    sessionID,
				State: session.StateIdle,
			})
		}
	}

	bridge := &stt.GeminiBridge{
		APIKey:     h.cfg.GeminiAPIKey,
		APIVersion: h.cfg.GeminiAPIVersion,
	}
	liveClient, err := bridge.NewClient(ctx)
	if err != nil {
		writeWSError(clientWS, "failed to create Gemini client: "+err.Error())
		return
	}

	var (
		liveSession *genai.Session
		writeMu     sync.Mutex
		receiveWg   sync.WaitGroup
		flow        *orchestrator.Service
	)
	if h.cfg.Interrupts != nil {
		h.cfg.Interrupts.Register(sessionID, func(reason string) {
			if flow != nil {
				flow.CancelActiveRequest(reason)
			}
		})
		defer h.cfg.Interrupts.Unregister(sessionID)
	}

	closeSession := func() {
		cancel()
		if flow != nil {
			flow.CancelActiveRequest("Session closed. Active task canceled.")
		}
		if liveSession != nil {
			_ = liveSession.Close()
		}
	}
	defer closeSession()

	for {
		var envelope ClientEnvelope
		if err := clientWS.ReadJSON(&envelope); err != nil {
			break
		}

		switch envelope.Type {
		case "setup":
			if liveSession != nil {
				writeWSError(clientWS, "session already initialized")
				continue
			}
			if envelope.Setup == nil {
				writeWSError(clientWS, "missing setup payload")
				continue
			}
			if strings.TrimSpace(envelope.SessionID) != "" {
				requested := strings.TrimSpace(envelope.SessionID)
				if sessionIDFromQuery && requested != sessionID {
					writeWSError(clientWS, "session id mismatch between query and setup payload")
					return
				}
				if h.cfg.Sessions != nil {
					if _, ok := h.cfg.Sessions.Get(sessionID); !ok {
						if requested != sessionID {
							if _, requestedOK := h.cfg.Sessions.Get(requested); !requestedOK {
								writeWSError(clientWS, "session not found")
								return
							}
						} else {
							writeWSError(clientWS, "session not found")
							return
						}
					}
				}
				if requested != sessionID {
					if h.cfg.Interrupts != nil {
						h.cfg.Interrupts.Unregister(sessionID)
						h.cfg.Interrupts.Register(requested, func(reason string) {
							if flow != nil {
								flow.CancelActiveRequest(reason)
							}
						})
						defer h.cfg.Interrupts.Unregister(requested)
					}
					sessionID = requested
				}
			}

			selectedAvatar := h.resolveAvatar(envelope.Setup.AvatarID)
			envelope.Setup.AvatarID = selectedAvatar.ID
			if strings.TrimSpace(envelope.Setup.SystemInstruction) == "" {
				envelope.Setup.SystemInstruction = selectedAvatar.SystemInstruction
			}

			// Initialize MCP Client and inject tools if configured
			var mcpClient *mcp.Client
			mcpEndpoint := strings.TrimSpace(selectedAvatar.Endpoint)
			if mcpEndpoint == "" && h.cfg.MCP != nil {
				mcpEndpoint = strings.TrimSpace(h.cfg.MCP.SSEURL)
			}
			if selectedAvatar.QuerySource == "mcp" && mcpEndpoint != "" {
				mClient, err := mcp.NewSSEClient(mcpEndpoint)
				if err == nil {
					mcpClient = mClient
					toolsRes, err := mcpClient.ListTools(ctx)
					if err == nil && toolsRes != nil {
						envelope.Setup.Tools = append(envelope.Setup.Tools, mcp.ToGenAITools(toolsRes.Tools)...)
					}
				} else {
					writer := &clientConnWriter{ws: clientWS, writeMu: &writeMu}
					writer.SendStatus("Warning: MCP connection failed - " + err.Error())
				}
			}

			liveSession, err = bridge.Connect(ctx, liveClient, envelope.Setup)
			if err != nil {
				writeWSError(clientWS, "failed to connect Gemini Live: "+err.Error())
				if mcpClient != nil {
					mcpClient.Close()
				}
				return
			}

			writer := &clientConnWriter{ws: clientWS, writeMu: &writeMu}
			llmClient := h.resolveLLMClient(selectedAvatar, mcpEndpoint)
			flow = &orchestrator.Service{
				SessionID:         sessionID,
				Sessions:          h.cfg.Sessions,
				Setup:             envelope.Setup,
				Session:           liveSession,
				Writer:            writer,
				LLM:               llmClient,
				ProcessingMessage: h.cfg.ProcessingMessage,
				ErrorMessage:      h.cfg.ErrorMessage,
			}

			receiveWg.Add(1)
			go func() {
				defer receiveWg.Done()
				for {
					message, err := liveSession.Receive()
					if err != nil {
						if ctx.Err() == nil {
							writer.SendStatus("Gemini Live disconnected.")
							writeWSError(clientWS, err.Error())
						}
						return
					}

					flow.HandleLiveServerMessage(message)

					// Check for ToolCall from Gemini
					var isIntercepted bool
					if message.ToolCall != nil && len(message.ToolCall.FunctionCalls) > 0 && mcpClient != nil {
						// Intercept and call the MCP tools
						responses := make([]*genai.FunctionResponse, 0)
						for _, call := range message.ToolCall.FunctionCalls {
							res, callErr := mcpClient.CallTool(ctx, call.Name, call.Args)
							if callErr != nil {
								responses = append(responses, &genai.FunctionResponse{
									ID:       call.ID,
									Name:     call.Name,
									Response: map[string]any{"error": callErr.Error()},
								})
							} else {
								responses = append(responses, &genai.FunctionResponse{
									ID:       call.ID,
									Name:     call.Name,
									Response: mcp.ResultToMap(res),
								})
							}
						}
						// Send tool response back to Gemini immediately
						liveSession.SendToolResponse(genai.LiveToolResponseInput{
							FunctionResponses: responses,
						})
						isIntercepted = true
					}

					// We only send to the browser client if not intercepted, or maybe we want to send it anyway?
					// Usually we don't need the browser client to handle backend tools.
					if !isIntercepted {
						writeMu.Lock()
						err = clientWS.WriteJSON(ServerEnvelope{Type: "live", Message: message})
						writeMu.Unlock()
						if err != nil {
							cancel()
							return
						}
					}
				}
			}()

			writeMu.Lock()
			err = clientWS.WriteJSON(ServerEnvelope{
				Type: "ready",
				Message: map[string]any{
					"sessionId": sessionID,
					"avatarId":  selectedAvatar.ID,
					"model":     envelope.Setup.Model,
					"voice":     envelope.Setup.Voice,
					"version":   h.cfg.GeminiAPIVersion,
				},
			})
			writeMu.Unlock()
			if err != nil {
				return
			}
			writer.SendStatus("Gemini Live ready. Waiting for speech.")
			if h.cfg.Sessions != nil {
				h.cfg.Sessions.Update(sessionID, func(current *session.Session) {
					current.State = session.StateListening
					current.SystemPrompt = envelope.Setup.SystemInstruction
				})
			}

			// Add cleanup for mcpClient to closeSession
			originalClose := closeSession
			closeSession = func() {
				if mcpClient != nil {
					mcpClient.Close()
				}
				originalClose()
			}

		case "text":
			if flow == nil {
				writeWSError(clientWS, "send setup first")
				continue
			}
			if strings.TrimSpace(envelope.Text) == "" {
				continue
			}
			flow.StartTextTurn(strings.TrimSpace(envelope.Text))

		case "audio_chunk":
			if liveSession == nil {
				writeWSError(clientWS, "send setup first")
				continue
			}
			if envelope.Audio == "" {
				continue
			}
			audioData, err := base64.StdEncoding.DecodeString(envelope.Audio)
			if err != nil {
				writeWSError(clientWS, "invalid audio payload: "+err.Error())
				continue
			}
			err = liveSession.SendRealtimeInput(genai.LiveRealtimeInput{
				Audio: &genai.Blob{
					MIMEType: "audio/pcm",
					Data:     audioData,
				},
			})
			if err != nil {
				writeWSError(clientWS, "failed to send audio: "+err.Error())
			}

		case "audio_end":
			if liveSession == nil {
				writeWSError(clientWS, "send setup first")
				continue
			}
			err = liveSession.SendRealtimeInput(genai.LiveRealtimeInput{
				AudioStreamEnd: true,
			})
			if err != nil {
				writeWSError(clientWS, "failed to end audio stream: "+err.Error())
			}

		case "tool_response":
			// If it's a request to execute a tool, we might need to intercept it.
			// But wait, Gemini Live Server Envelope for tool_call comes from the server.
			// Wait, if it comes from the client, we just forward it to Gemini.
			// The actual function call is initiated by the model, so it comes from liveSession.Receive()
			// Let's modify the LiveServerMessage handler instead.
			if liveSession == nil {
				writeWSError(clientWS, "send setup first")
				continue
			}
			responses := make([]*genai.FunctionResponse, 0, len(envelope.FunctionReply))
			for _, reply := range envelope.FunctionReply {
				payload := map[string]any{}
				if reply.Error != "" {
					payload["error"] = reply.Error
				}
				for key, value := range reply.Result {
					payload[key] = value
				}
				responses = append(responses, &genai.FunctionResponse{
					ID:       reply.ID,
					Name:     reply.Name,
					Response: payload,
				})
			}
			err = liveSession.SendToolResponse(genai.LiveToolResponseInput{
				FunctionResponses: responses,
			})
			if err != nil {
				writeWSError(clientWS, "failed to send tool response: "+err.Error())
			}

		default:
			writeWSError(clientWS, "unsupported message type: "+envelope.Type)
		}
	}

	closeSession()
	if h.cfg.Sessions != nil {
		h.cfg.Sessions.Update(sessionID, func(current *session.Session) {
			current.State = session.StateIdle
		})
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		receiveWg.Wait()
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func (h *LiveHandler) resolveAvatar(requested string) avatarconfig.Avatar {
	if h.cfg.AvatarStore != nil {
		if avatar, ok := h.cfg.AvatarStore.Get(strings.TrimSpace(requested)); ok && avatar.Enabled {
			return avatar
		}
		return h.cfg.AvatarStore.Default()
	}
	return avatarconfig.Avatar{
		ID:          "default",
		DisplayName: "Default",
		Enabled:     true,
		QuerySource: "llm",
	}
}

func (h *LiveHandler) resolveLLMClient(avatar avatarconfig.Avatar, mcpEndpoint string) llm.Client {
	switch avatar.QuerySource {
	case "mcp":
		return llm.NewMCPQueryClient(llm.MCPQueryConfig{
			Endpoint:              firstNonEmpty(mcpEndpoint, avatar.Endpoint),
			ToolName:              avatar.MCPToolName,
			ToolArgument:          avatar.MCPToolArgument,
			SystemPrompt:          avatar.SystemInstruction,
			EnableSensitiveLookup: h.cfg.MCP != nil && h.cfg.MCP.EnableSensitiveLookup,
			EnableSensitiveGuard:  h.cfg.MCP == nil || h.cfg.MCP.EnableSensitiveGuard,
		})
	default:
		return llm.NewOpenAICompatClient(h.cfg.LLM)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func writeWSError(ws *websocket.Conn, message string) {
	_ = ws.WriteJSON(ServerEnvelope{Type: "error", Error: message})
}
