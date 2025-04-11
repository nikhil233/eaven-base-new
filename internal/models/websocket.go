package models

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// GlobalHub is a singleton instance of the Hub
var GlobalHub *Hub
var hubOnce sync.Once

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
	TeamChannels map[string]map[string]map[string][]*Client

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
		TeamChannels: make(map[string]map[string]map[string][]*Client),
	}
}

// GetHub returns the singleton instance of the Hub
func GetHub() *Hub {
	hubOnce.Do(func() {
		GlobalHub = NewHub()
		go GlobalHub.Run()
	})
	return GlobalHub
}

// Run starts the hub's message handling
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			h.Clients[client] = true

			// Initialize maps if they don't exist
			if _, exists := h.TeamChannels[client.TeamID]; !exists {
				h.TeamChannels[client.TeamID] = make(map[string]map[string][]*Client)
			}
			if _, exists := h.TeamChannels[client.TeamID]["userConnection"]; !exists {
				h.TeamChannels[client.TeamID]["userConnection"] = make(map[string][]*Client)
			}
			if _, exists := h.TeamChannels[client.TeamID]["userConnection"][client.UserID]; !exists {
				h.TeamChannels[client.TeamID]["userConnection"][client.UserID] = make([]*Client, 0)
			}

			// Add client to the slice
			h.TeamChannels[client.TeamID]["userConnection"][client.UserID] = append(
				h.TeamChannels[client.TeamID]["userConnection"][client.UserID],
				client,
			)
			h.mu.Unlock()

		case client := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)

				// Remove client from the slice
				if teamChannels, exists := h.TeamChannels[client.TeamID]; exists {
					if userConnections, exists := teamChannels["userConnection"]; exists {
						if clients, exists := userConnections[client.UserID]; exists {
							// Find and remove the client from the slice
							for i, c := range clients {
								if c == client {
									// Remove the client from the slice
									h.TeamChannels[client.TeamID]["userConnection"][client.UserID] = append(
										clients[:i],
										clients[i+1:]...,
									)
									break
								}
							}

							// If the slice is empty, remove the user entry
							if len(h.TeamChannels[client.TeamID]["userConnection"][client.UserID]) == 0 {
								delete(h.TeamChannels[client.TeamID]["userConnection"], client.UserID)
							}
						}

						// If no users left, remove the userConnection entry
						if len(h.TeamChannels[client.TeamID]["userConnection"]) == 0 {
							delete(h.TeamChannels[client.TeamID], "userConnection")
						}
					}

					// If no connections left, remove the team entry
					if len(h.TeamChannels[client.TeamID]) == 0 {
						delete(h.TeamChannels, client.TeamID)
					}
				}

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

	if teamChannels, exists := h.TeamChannels[teamID]; exists {
		if userConnections, exists := teamChannels["userConnection"]; exists {
			for _, clients := range userConnections {
				for _, client := range clients {
					select {
					case client.Send <- message:
					default:
						close(client.Send)
						delete(h.Clients, client)
					}
				}
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

// GetUserConnections returns all active connections for a user in a team
func (h *Hub) GetUserConnections(teamID string, userID string) []*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if teamChannels, exists := h.TeamChannels[teamID]; exists {
		if userConnections, exists := teamChannels["userConnection"]; exists {
			if clients, exists := userConnections[userID]; exists {
				return clients
			}
		}
	}
	return nil
}

// IsUserConnected checks if a user has any active connections in a team
func (h *Hub) IsUserConnected(teamID string, userID string) bool {
	clients := h.GetUserConnections(teamID, userID)
	return len(clients) > 0
}

// SendMessageToUser sends a message to all connections of a user in a team
func (h *Hub) SendMessageToUser(teamID string, userID string, message []byte) bool {
	clients := h.GetUserConnections(teamID, userID)
	if len(clients) == 0 {
		return false
	}

	success := false
	for _, client := range clients {
		select {
		case client.Send <- message:
			success = true
		default:
			// Message could not be sent to this client
		}
	}
	return success
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
