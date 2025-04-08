package authRoute

import (
	"github.com/gorilla/mux"
	"github.com/nikhil/eaven/internal/handlers"
	"github.com/nikhil/eaven/internal/middleware"
	services "github.com/nikhil/eaven/internal/service/auth"
)

func RegisterAuthRoutes(router *mux.Router) {
	authService := services.NewAuthService()
	authHandler := handlers.NewAuthHandler(authService)

	// Public routes without auth middleware
	publicRouter := router.PathPrefix("/auth").Subrouter()
	publicRouter.Use(middleware.ResponseWrapperMiddleware)
	publicRouter.HandleFunc("/signup", authHandler.Signup).Methods("POST")
	publicRouter.HandleFunc("/login", authHandler.Login).Methods("POST")
}
