package userRoutes

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/nikhil/eaven/internal/middleware"
	profileService "github.com/nikhil/eaven/internal/service/users"
)

func UserProfileRoutes(router *mux.Router) {
	profileService := profileService.NewProfileService()

	// Protected routes requiring authentication
	protectedRouter := router.PathPrefix("/user").Subrouter()
	protectedRouter.Use(middleware.AuthMiddleware, middleware.ResponseWrapperMiddleware)

	// User profile routes
	protectedRouter.HandleFunc("/profile", profileService.GetUserProfile).Methods(http.MethodGet, http.MethodOptions)
	protectedRouter.HandleFunc("/profile", profileService.UpdateUserProfile).Methods(http.MethodPut, http.MethodOptions)
}
