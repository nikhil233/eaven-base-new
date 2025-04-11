package models

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	// Registered clients.
	Clients map[*Client]bool

	// Inbound messages from the clients.
	Broadcast chan []byte

	// Register requests from the clients.
	Register chan *Client

	// Unregister requests from clients.
	Unregister chan *Client

	// Team-based message routing
	TeamChannels map[string]map[*Client]bool

	// Mutex for thread-safe operations
	mu sync.RWMutex
}

// Client represents a WebSocket connection
type Client struct {
	Hub *Hub

	// The websocket connection.
	Conn *websocket.Conn

	// Buffered channel of outbound messages.
	Send chan []byte

	// User information
	UserID string
	TeamID string
}

// Message represents the structure of WebSocket messages
type Message struct {
	Type    string `json:"type"`
	Content string `json:"content"`
	TeamID  string `json:"team_id"`
	UserID  string `json:"user_id"`
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		Broadcast:    make(chan []byte),
		Register:     make(chan *Client),
		Unregister:   make(chan *Client),
		Clients:      make(map[*Client]bool),
		TeamChannels: make(map[string]map[*Client]bool),
	}
}

// Run starts the hub's message handling
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			h.Clients[client] = true
			if _, exists := h.TeamChannels[client.TeamID]; !exists {
				h.TeamChannels[client.TeamID] = make(map[*Client]bool)
			}
			h.TeamChannels[client.TeamID][client] = true
			h.mu.Unlock()

		case client := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				delete(h.TeamChannels[client.TeamID], client)
				close(client.Send)
			}
			h.mu.Unlock()

		case message := <-h.Broadcast:
			h.mu.RLock()
			for client := range h.Clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(h.Clients, client)
					delete(h.TeamChannels[client.TeamID], client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastToTeam sends a message to all clients in a specific team
func (h *Hub) BroadcastToTeam(teamID string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if teamClients, exists := h.TeamChannels[teamID]; exists {
		for client := range teamClients {
			select {
			case client.Send <- message:
			default:
				close(client.Send)
				delete(h.Clients, client)
				delete(h.TeamChannels[teamID], client)
			}
		}
	}
}

// ReadPump pumps messages from the WebSocket connection to the hub
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// log error
			}
			break
		}

		// Parse the message
		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		// Set the user and team IDs from the client
		msg.UserID = c.UserID
		msg.TeamID = c.TeamID

		// Re-marshal the message
		messageBytes, err := json.Marshal(msg)
		if err != nil {
			continue
		}

		// Broadcast to team
		c.Hub.BroadcastToTeam(c.TeamID, messageBytes)
	}
}

// WritePump pumps messages from the hub to the WebSocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed
	maxMessageSize = 512
)
