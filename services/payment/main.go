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
	shutdown, err := common.InitTelemetry("payment-service")
	if err != nil {
		log.Printf("Failed to initialize telemetry: %v", err)
	}
	defer shutdown()

	// Load processor configuration
	if err := LoadProcessorConfig("config.yaml"); err != nil {
		log.Fatalf("Failed to load processor config: %v", err)
	}
	log.Printf("Loaded %d payment processors", len(processors))

	// Initialize transaction storage
	InitTransactionStorage()

	// Create router
	r := mux.NewRouter()

	// Apply middleware
	r.Use(common.LoggingMiddleware)
	r.Use(common.CorrelationIDMiddleware)
	r.Use(common.TracingMiddleware("payment-service"))
	r.Use(common.CORSMiddleware)

	// Register routes
	RegisterRoutes(r)

	// Start server
	port := getPort()
	log.Printf("Payment Service starting on port %d", port)
	log.Printf("Available processors: PuppyPay, KittyCard, DoggieCoin")

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
	return 8081
}
