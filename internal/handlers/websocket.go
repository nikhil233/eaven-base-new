package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/nikhil/eaven/internal/middleware"
	"github.com/nikhil/eaven/internal/models"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // In production, replace with proper origin checking
	},
}

// WebSocketHandler handles WebSocket connections
type WebSocketHandler struct {
	hub *models.Hub
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler() *WebSocketHandler {
	// Use the singleton Hub instance
	hub := models.GetHub()
	return &WebSocketHandler{hub: hub}
}

// HandleWebSocket handles incoming WebSocket connections
func (h *WebSocketHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Get user information from context (set by auth middleware)
	claims, ok := r.Context().Value(middleware.UserContextKey).(jwt.MapClaims)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userIDFloat := claims["user_id"].(float64) // JWT numbers are decoded as float64
	userID := strconv.FormatInt(int64(userIDFloat), 10)
	teamID := r.URL.Query().Get("team_id")
	if teamID == "" {
		http.Error(w, "Team ID is required", http.StatusBadRequest)
		return
	}

	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading connection: %v", err)
		return
	}

	client := &models.Client{
		Hub:    h.hub,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		UserID: userID,
		TeamID: teamID,
	}

	// Register the client with the hub
	h.hub.Register <- client

	// Start goroutines for reading and writing messages
	go client.WritePump()
	go client.ReadPump()

}
