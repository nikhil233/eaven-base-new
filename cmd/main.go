package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/nikhil/eaven/internal/database.go"
	"github.com/nikhil/eaven/internal/models"
	"github.com/nikhil/eaven/internal/routes"
)

func main() {
	database.InitDB()

	// Initialize the WebSocket Hub
	models.GetHub()

	router := routes.RegisterAllRoutes()

	fmt.Println("Server is running on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", router))
}
