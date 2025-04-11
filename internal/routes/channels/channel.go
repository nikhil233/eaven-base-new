package channnelRoutes

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/nikhil/eaven/internal/middleware"
	channelService "github.com/nikhil/eaven/internal/service/channels"
	messageService "github.com/nikhil/eaven/internal/service/messages"
)

func ChannelRoutes(router *mux.Router) {
	channelService := channelService.NewChannelService()
	messageService := messageService.NewMessageService()

	// Protected routes requiring authentication
	protectedRouter := router.PathPrefix("/channel").Subrouter()
	protectedRouter.Use(middleware.AuthMiddleware, middleware.ResponseWrapperMiddleware)

	// Team routes
	protectedRouter.HandleFunc("/create", channelService.CreateChannel).Methods(http.MethodPost)
	// protectedRouter.HandleFunc("/all", channelService.GetUserTeams).Methods(http.MethodGet)
	protectedRouter.HandleFunc("/get/{id}", channelService.GetChannel).Methods(http.MethodGet)
	// protectedRouter.HandleFunc("/update/{id}", channelService.UpdateTeam).Methods(http.MethodPut)
	// protectedRouter.HandleFunc("/{team_id}/channels", channelService.GetUserTeams).Methods(http.MethodGet)

	protectedRouter.HandleFunc("/{channel_id}/join", channelService.SubscribeChannel).Methods(http.MethodPost)
	protectedRouter.HandleFunc("/message", messageService.SendMessage).Methods(http.MethodPost)
}
