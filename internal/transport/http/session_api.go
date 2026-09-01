package httptransport

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/asccclass/sherryserver/internal/orchestrator"
	"github.com/asccclass/sherryserver/internal/session"
	"github.com/google/uuid"
)

type SessionAPI struct {
	Sessions   *session.Manager
	Interrupts *orchestrator.InterruptController
}

func NewSessionAPI(sessions *session.Manager, interrupts *orchestrator.InterruptController) *SessionAPI {
	return &SessionAPI{
		Sessions:   sessions,
		Interrupts: interrupts,
	}
}

type createSessionRequest struct {
	UserID string `json:"userId"`
}

func (api *SessionAPI) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req createSessionRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	sessionID := uuid.NewString()
	s := &session.Session{
		ID:     sessionID,
		UserID: strings.TrimSpace(req.UserID),
		State:  session.StateIdle,
	}
	api.Sessions.Upsert(s)
	api.Sessions.Publish(sessionID, session.Event{
		Type:      "session.created",
		SessionID: sessionID,
		Payload: map[string]any{
			"id":    sessionID,
			"state": session.StateIdle,
		},
	})

	writeJSON(w, http.StatusCreated, map[string]any{
		"ok":        true,
		"sessionId": sessionID,
		"state":     session.StateIdle,
	})
}

func (api *SessionAPI) GetSession(w http.ResponseWriter, r *http.Request) {
	sessionID := sessionIDFromPath(r.URL.Path)
	if sessionID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing session id")
		return
	}

	current, ok := api.Sessions.Get(sessionID)
	if !ok {
		writeJSONError(w, http.StatusNotFound, "session not found")
		return
	}

	writeJSON(w, http.StatusOK, current)
}

func (api *SessionAPI) SessionEvents(w http.ResponseWriter, r *http.Request) {
	sessionID := sessionIDFromEventsPath(r.URL.Path)
	if sessionID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing session id")
		return
	}

	if _, ok := api.Sessions.Get(sessionID); !ok {
		writeJSONError(w, http.StatusNotFound, "session not found")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	events, unsubscribe := api.Sessions.Subscribe(sessionID)
	defer unsubscribe()

	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		case event, ok := <-events:
			if !ok {
				return
			}
			payload, err := json.Marshal(event)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: %s\n", event.Type)
			fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
		}
	}
}

func (api *SessionAPI) InterruptSession(w http.ResponseWriter, r *http.Request) {
	sessionID := sessionIDFromInterruptPath(r.URL.Path)
	if sessionID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing session id")
		return
	}

	current, ok := api.Sessions.Get(sessionID)
	if !ok {
		writeJSONError(w, http.StatusNotFound, "session not found")
		return
	}

	reason := "Interrupted from HTTP API."
	if api.Interrupts != nil {
		if canceled := api.Interrupts.CancelSession(sessionID, reason); !canceled {
			writeJSONError(w, http.StatusConflict, "session is not actively running")
			return
		}
	}

	api.Sessions.Update(sessionID, func(s *session.Session) {
		s.State = session.StateInterrupted
		s.LastError = reason
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"sessionId":        sessionID,
		"state":            session.StateInterrupted,
		"interruptedTurnId": current.CurrentTurnID,
	})
}

func sessionIDFromPath(path string) string {
	const prefix = "/api/session/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	tail := strings.TrimPrefix(path, prefix)
	if tail == "" || strings.Contains(tail, "/") {
		return ""
	}
	return tail
}

func sessionIDFromInterruptPath(path string) string {
	const prefix = "/api/session/"
	const suffix = "/interrupt"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return ""
	}
	tail := strings.TrimPrefix(path, prefix)
	tail = strings.TrimSuffix(tail, suffix)
	tail = strings.TrimSuffix(tail, "/")
	if tail == "" || strings.Contains(tail, "/") {
		return ""
	}
	return tail
}

func sessionIDFromEventsPath(path string) string {
	const prefix = "/api/session/"
	const suffix = "/events"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return ""
	}
	tail := strings.TrimPrefix(path, prefix)
	tail = strings.TrimSuffix(tail, suffix)
	tail = strings.TrimSuffix(tail, "/")
	if tail == "" || strings.Contains(tail, "/") {
		return ""
	}
	return tail
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"ok":    false,
		"error": message,
	})
}
