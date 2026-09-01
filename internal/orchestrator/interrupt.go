package orchestrator

import (
	"context"
	"fmt"
	"sync"
)

type CancelRegistry interface {
	CancelSession(sessionID string, reason string) bool
	Register(sessionID string, cancel func(string))
	Unregister(sessionID string)
}

type InterruptController struct {
	mu        sync.RWMutex
	callbacks map[string]func(string)
}

func NewInterruptController() *InterruptController {
	return &InterruptController{
		callbacks: make(map[string]func(string)),
	}
}

func (c *InterruptController) Register(sessionID string, cancel func(string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.callbacks[sessionID] = cancel
}

func (c *InterruptController) Unregister(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.callbacks, sessionID)
}

func (c *InterruptController) CancelSession(sessionID string, reason string) bool {
	c.mu.RLock()
	cancel, ok := c.callbacks[sessionID]
	c.mu.RUnlock()
	if !ok {
		return false
	}
	cancel(reason)
	return true
}

func (c *InterruptController) Cancel(ctx context.Context, sessionID string, reason string) error {
	_ = ctx
	if ok := c.CancelSession(sessionID, reason); !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}
	return nil
}
