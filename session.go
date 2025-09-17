package main

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type SessionManager struct {
	sessions sync.Map
}

type Session struct {
	ID         string
	Conn       *websocket.Conn
	Game       *Game
	InputChan  chan InputMessage
	Done       chan struct{}
	CreatedAt  time.Time
	LastActive time.Time
	mu         sync.Mutex
}

func generateSessionID() string {
	return uuid.New().String()
}

func (s *Session) UpdateLastActive() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastActive = time.Now()
}

func (sm *SessionManager) GetActiveSessions() []map[string]interface{} {
	var sessions []map[string]interface{}
	sm.sessions.Range(func(key, value interface{}) bool {
		session := value.(*Session)
		sessions = append(sessions, map[string]interface{}{
			"id":         session.ID,
			"created":    session.CreatedAt.Format(time.RFC3339),
			"lastActive": session.LastActive.Format(time.RFC3339),
			"clientFPS":  session.Game.ClientFPS,
			"latency":    session.Game.AvgLatency,
		})
		return true
	})
	return sessions
}