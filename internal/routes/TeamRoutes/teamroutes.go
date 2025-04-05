package teamroutes

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/nikhil/eaven/internal/middleware"
	teamService "github.com/nikhil/eaven/internal/service/team"
)

func TeamRoutes(router *mux.Router) {

	// profileService := profileService.NewProfileService()
	teamService := teamService.NewTeamService()

	publicRouter := router.PathPrefix("/team").Subrouter()
	publicRouter.Use(middleware.AuthMiddleware, middleware.ResponseWrapperMiddleware)
	publicRouter.HandleFunc("/create", teamService.CreateTeam).Methods(http.MethodPost)
	publicRouter.HandleFunc("/all", teamService.GetUserTeams).Methods(http.MethodGet)
	publicRouter.HandleFunc("/get/{id}", teamService.GetTeam).Methods(http.MethodGet)
	publicRouter.HandleFunc("/update/{id}", teamService.UpdateTeam).Methods(http.MethodPut)
	// publicRouter := router.PathPrefix("/user").Subrouter()
	// publicRouter.Use(middleware.AuthMiddleware, middleware.ResponseWrapperMiddleware)
	// publicRouter.HandleFunc("/profile", profileService.GetUserProfile).Methods(http.MethodGet)
	// publicRouter.HandleFunc("/profile", profileService.UpdateUserProfile).Methods(http.MethodPut)
}
