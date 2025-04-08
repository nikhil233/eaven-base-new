package teamroutes

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/nikhil/eaven/internal/middleware"
	teamService "github.com/nikhil/eaven/internal/service/team"
)

func TeamRoutes(router *mux.Router) {
	teamService := teamService.NewTeamService()

	// Protected routes requiring authentication
	protectedRouter := router.PathPrefix("/team").Subrouter()
	protectedRouter.Use(middleware.AuthMiddleware, middleware.ResponseWrapperMiddleware)

	// Team routes
	protectedRouter.HandleFunc("/create", teamService.CreateTeam).Methods(http.MethodPost)
	protectedRouter.HandleFunc("/all", teamService.GetUserTeams).Methods(http.MethodGet)
	protectedRouter.HandleFunc("/get/{id}", teamService.GetTeam).Methods(http.MethodGet)
	protectedRouter.HandleFunc("/update/{id}", teamService.UpdateTeam).Methods(http.MethodPut)
	protectedRouter.HandleFunc("/{team_id}/channels", teamService.GetUserTeams).Methods(http.MethodGet)
}
