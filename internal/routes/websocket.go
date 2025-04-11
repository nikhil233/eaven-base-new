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

	// WebSocket endpoint with authentication
	router.Handle("/ws", middleware.AuthMiddleware(http.HandlerFunc(wsHandler.HandleWebSocket))).Methods("GET")
}
