package shipping

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/cardinalhq/griffin-commerce-demo/common"
)

func Start() error {
	// Initialize telemetry
	shutdown, err := common.InitTelemetry("shipping-service")
	if err != nil {
		log.Printf("Failed to initialize telemetry: %v", err)
	}
	defer shutdown()

	// Load carrier configuration
	if err := LoadCarrierConfig("config.yaml"); err != nil {
		log.Fatalf("Failed to load carrier config: %v", err)
	}
	log.Printf("Loaded %d shipping carriers", len(carriers))

	// Initialize shipment storage
	InitShipmentStorage()

	// Create router
	r := mux.NewRouter()

	// Apply middleware
	r.Use(common.LoggingMiddleware)
	r.Use(common.CorrelationIDMiddleware)
	r.Use(common.TracingMiddleware("shipping-service"))
	r.Use(common.CORSMiddleware)

	// Register routes
	RegisterRoutes(r)

	// Start server
	port := getPort()
	log.Printf("Shipping Service starting on port %d", port)
	log.Printf("Available carriers: PonyExpress, AvianAir, CatCarrier")

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: r,
	}

	if err := server.ListenAndServe(); err != nil {
		return fmt.Errorf("server failed to start: %w", err)
	}

	return nil
}

func getPort() int {
	if port := os.Getenv("PORT"); port != "" {
		var p int
		if _, err := fmt.Sscanf(port, "%d", &p); err != nil {
			log.Printf("Failed to parse port: %v", err)
			return 8084
		}
		return p
	}
	return 8084
}
