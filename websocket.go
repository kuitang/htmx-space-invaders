package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var sessionManager = &SessionManager{}

type InputMessage struct {
	Action  string
	Type    string
	FrameID string
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer conn.Close()

	// Create session
	session := &Session{
		ID:         generateSessionID(),
		Conn:       conn,
		Game:       NewGame(),
		InputChan:  make(chan InputMessage, 10),
		Done:       make(chan struct{}),
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
	}

	// Set session ID in game
	session.Game.SessionID = session.ID

	// Register session
	sessionManager.sessions.Store(session.ID, session)
	defer func() {
		sessionManager.sessions.Delete(session.ID)
		close(session.Done)
		log.Printf("Session %s ended", session.ID)
	}()

	log.Printf("Session %s started", session.ID)

	inputChan := session.InputChan

	// Start goroutine to read messages
	go func() {
		defer close(inputChan)
		for {
			// First read raw message to debug
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("Unexpected close error: %v", err)
				} else {
					log.Printf("ReadMessage error: %v", err)
				}
				break
			}

			log.Printf("Raw message (type=%d): %s", messageType, string(message))

			// Parse JSON message from HTMX WebSocket extension
			var rawMsg map[string]interface{}
			if err := json.Unmarshal(message, &rawMsg); err != nil {
				log.Printf("JSON unmarshal error: %v", err)
				continue
			}

			// Extract form fields (everything except HEADERS)
			msg := InputMessage{}
			if action, ok := rawMsg["action"].(string); ok {
				msg.Action = action
			}
			if inputType, ok := rawMsg["type"].(string); ok {
				msg.Type = inputType
			}
			if frameID, ok := rawMsg["frameId"].(string); ok {
				msg.FrameID = frameID
			}

			log.Printf("Parsed message: action=%s, type=%s, frameId=%s", msg.Action, msg.Type, msg.FrameID)
			inputChan <- msg
		}
	}()

	// Game loop - 60 FPS
	lastFrame := time.Now()
	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-inputChan:
			if !ok {
				return
			}
			handled := false
			if msg.FrameID != "" {
				if frameID, err := strconv.ParseUint(msg.FrameID, 10, 64); err == nil {
					session.Game.ProcessFrameAck(frameID)
					handled = true
				}
			}

			switch msg.Action {
			case "left", "right", "shoot":
				session.Game.ProcessInput(msg.Action, msg.Type)
				handled = true
			}

			if handled {
				session.UpdateLastActive()
			}

		case <-ticker.C:
			// Calculate delta time for real-time movement
			now := time.Now()
			deltaTime := now.Sub(lastFrame).Seconds()
			lastFrame = now

			// Update game state
			session.Game.Update(deltaTime)

			// Increment frame ID and track send time
			session.Game.FrameID++
			session.Game.FrameSentTimes[session.Game.FrameID] = now

			// Render HTML with OOB swaps
			html := session.Game.RenderHTML()

			// Send HTML to client
			err := conn.WriteMessage(websocket.TextMessage, []byte(html))
			if err != nil {
				log.Println("write:", err)
				return
			}
		}
	}
}
