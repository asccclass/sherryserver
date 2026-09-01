package session

import (
	"sync"
	"time"
)

type Manager struct {
	mu          sync.RWMutex
	sessions    map[string]*Session
	subscribers map[string]map[chan Event]struct{}
}

func NewManager() *Manager {
	return &Manager{
		sessions:    make(map[string]*Session),
		subscribers: make(map[string]map[chan Event]struct{}),
	}
}

func (m *Manager) Upsert(s *Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s.UpdatedAt = time.Now()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = s.UpdatedAt
	}
	m.sessions[s.ID] = s
	event := Event{
		Type:      "session.updated",
		SessionID: s.ID,
		Payload:   snapshotPayload(s),
	}
	m.broadcastLocked(s.ID, event)
}

func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return s, ok
}

func (m *Manager) Update(id string, mutate func(*Session)) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[id]
	if !ok {
		s = &Session{ID: id}
		m.sessions[id] = s
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	mutate(s)
	s.UpdatedAt = time.Now()
	event := Event{
		Type:      "session.updated",
		SessionID: s.ID,
		TurnID:    s.CurrentTurnID,
		Payload:   snapshotPayload(s),
	}
	m.broadcastLocked(id, event)
	return s
}

func (m *Manager) Subscribe(sessionID string) (<-chan Event, func()) {
	ch := make(chan Event, 16)

	m.mu.Lock()
	if _, ok := m.subscribers[sessionID]; !ok {
		m.subscribers[sessionID] = make(map[chan Event]struct{})
	}
	m.subscribers[sessionID][ch] = struct{}{}
	if s, ok := m.sessions[sessionID]; ok {
		ch <- Event{
			Type:      "session.snapshot",
			SessionID: sessionID,
			TurnID:    s.CurrentTurnID,
			Payload:   snapshotPayload(s),
		}
	}
	m.mu.Unlock()

	cancel := func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if subs, ok := m.subscribers[sessionID]; ok {
			if _, exists := subs[ch]; exists {
				delete(subs, ch)
				close(ch)
			}
			if len(subs) == 0 {
				delete(m.subscribers, sessionID)
			}
		}
	}

	return ch, cancel
}

func (m *Manager) Publish(sessionID string, event Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.broadcastLocked(sessionID, event)
}

func (m *Manager) broadcastLocked(sessionID string, event Event) {
	subs := m.subscribers[sessionID]
	for ch := range subs {
		select {
		case ch <- event:
		default:
		}
	}
}

func snapshotPayload(s *Session) map[string]any {
	return map[string]any{
		"id":               s.ID,
		"userId":           s.UserID,
		"state":            s.State,
		"recognizedText":   s.RecognizedText,
		"systemPrompt":     s.SystemPrompt,
		"lastError":        s.LastError,
		"llmReply":         s.LLMReply,
		"liveTalkingJobId": s.LiveTalkingJobID,
		"liveKitRoom":      s.LiveKitRoom,
		"currentTurnId":    s.CurrentTurnID,
		"createdAt":        s.CreatedAt,
		"updatedAt":        s.UpdatedAt,
	}
}
