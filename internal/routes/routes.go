package routes

import (
	"github.com/gorilla/mux"
	authRoute "github.com/nikhil/eaven/internal/routes/Auth"
	userRoutes "github.com/nikhil/eaven/internal/routes/user"
)

// List of all route registration functions
var routeModules = []func(*mux.Router){
	authRoute.RegisterAuthRoutes,
	userRoutes.UserProfileRoutes,
}

// Register all routes dynamically
func RegisterAllRoutes() *mux.Router {
	router := mux.NewRouter()

	for _, register := range routeModules {
		register(router)
	}

	return router
}
