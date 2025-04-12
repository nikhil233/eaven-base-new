package routes

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/nikhil/eaven/internal/handlers"
	"github.com/nikhil/eaven/internal/middleware"
)

// RegisterWebSocketRoutes registers all WebSocket related routes
func RegisterWebSocketRoutes(router *mux.Router) {
	wsHandler := handlers.NewWebSocketHandler()

	// WebSocket endpoint with authentication via query parameter
	router.Handle("/ws", middleware.WebSocketAuthMiddleware(http.HandlerFunc(wsHandler.HandleWebSocket))).Methods("GET", "OPTIONS")
}
