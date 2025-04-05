package userRoutes

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/nikhil/eaven/internal/middleware"
	profileService "github.com/nikhil/eaven/internal/service/users"
)

func UserProfileRoutes(router *mux.Router) {

	profileService := profileService.NewProfileService()

	publicRouter := router.PathPrefix("/user").Subrouter()
	publicRouter.Use(middleware.AuthMiddleware, middleware.ResponseWrapperMiddleware)
	publicRouter.HandleFunc("/profile", profileService.GetUserProfile).Methods(http.MethodGet)
	publicRouter.HandleFunc("/profile", profileService.UpdateUserProfile).Methods(http.MethodPut)
}
