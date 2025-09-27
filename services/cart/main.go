package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/griffincommerce/demo/common"
)

func main() {
	// Initialize telemetry
	shutdown, err := common.InitTelemetry("cart-service")
	if err != nil {
		log.Printf("Failed to initialize telemetry: %v", err)
	}
	defer shutdown()

	// Initialize cart storage
	InitCartStorage()

	// Initialize product client
	catalogURL := os.Getenv("CATALOG_SERVICE_URL")
	if catalogURL == "" {
		catalogURL = "http://localhost:8080"
	}
	InitProductClient(catalogURL)

	// Create router
	r := mux.NewRouter()

	// Apply middleware
	r.Use(common.LoggingMiddleware)
	r.Use(common.CorrelationIDMiddleware)
	r.Use(common.TracingMiddleware("cart-service"))
	r.Use(common.CORSMiddleware)

	// Register routes
	RegisterRoutes(r)

	// Start server
	port := getPort()
	log.Printf("Cart Service starting on port %d", port)
	log.Printf("Using Catalog Service at: %s", catalogURL)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: r,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func getPort() int {
	if port := os.Getenv("PORT"); port != "" {
		var p int
		fmt.Sscanf(port, "%d", &p)
		return p
	}
	return 8082
}