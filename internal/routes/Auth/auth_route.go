package authRoute

import (
	"github.com/gorilla/mux"
	"github.com/nikhil/eaven/internal/handlers"
	services "github.com/nikhil/eaven/internal/service/auth"
)

func RegisterAuthRoutes(router *mux.Router) {
	authService := services.NewAuthService()
	authHandler := handlers.NewAuthHandler(authService)

	router.HandleFunc("/signup", authHandler.Signup).Methods("POST")
	router.HandleFunc("/login", authHandler.Login).Methods("GET")
}
