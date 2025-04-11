package routes

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/nikhil/eaven/internal/middleware"
	authRoute "github.com/nikhil/eaven/internal/routes/Auth"
	teamroutes "github.com/nikhil/eaven/internal/routes/TeamRoutes"
	channnelRoutes "github.com/nikhil/eaven/internal/routes/channels"
	userRoutes "github.com/nikhil/eaven/internal/routes/user"
)

// List of all route registration functions
var routeModules = []func(*mux.Router){
	authRoute.RegisterAuthRoutes,
	userRoutes.UserProfileRoutes,
	teamroutes.TeamRoutes,
	channnelRoutes.ChannelRoutes,
	RegisterWebSocketRoutes,
}

// Register all routes dynamically
func RegisterAllRoutes() *mux.Router {
	router := mux.NewRouter()

	// Apply CORS middleware to all routes
	router.Use(middleware.CORSMiddleware)

	// Apply route modules
	for _, register := range routeModules {
		register(router)
	}

	// Add a global OPTIONS handler for all routes
	router.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
		}
	}).Methods("OPTIONS")

	return router
}
