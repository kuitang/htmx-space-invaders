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

	game := NewGame()

	// Channel for incoming messages
	inputChan := make(chan InputMessage, 10)

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

	// Game loop - 144 FPS
	lastFrame := time.Now()
	ticker := time.NewTicker(time.Duration(1000/144) * time.Millisecond) // ~144 FPS
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-inputChan:
			if !ok {
				return
			}
			if msg.Action == "ack" && msg.FrameID != "" {
				// Handle frame acknowledgment
				if frameID, err := strconv.ParseUint(msg.FrameID, 10, 64); err == nil {
					game.ProcessFrameAck(frameID)
				}
			} else {
				game.ProcessInput(msg.Action, msg.Type)
			}

		case <-ticker.C:
			// Calculate delta time for real-time movement
			now := time.Now()
			deltaTime := now.Sub(lastFrame).Seconds()
			lastFrame = now

			// Update game state
			game.Update(deltaTime)

			// Increment frame ID and track send time
			game.FrameID++
			game.FrameSentTimes[game.FrameID] = now

			// Render HTML with OOB swaps
			html := game.RenderHTML()

			// Send HTML to client
			err := conn.WriteMessage(websocket.TextMessage, []byte(html))
			if err != nil {
				log.Println("write:", err)
				return
			}
		}
	}
}