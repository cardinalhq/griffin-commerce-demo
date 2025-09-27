package catalog

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
	shutdown, err := common.InitTelemetry("catalog-service")
	if err != nil {
		log.Printf("Failed to initialize telemetry: %v", err)
	}
	defer shutdown()

	// Load products from YAML
	if err := LoadProducts("products.yaml"); err != nil {
		return fmt.Errorf("failed to load products: %w", err)
	}
	log.Printf("Loaded %d products", GetProductCount())

	// Create router
	r := mux.NewRouter()

	// Apply middleware
	r.Use(common.LoggingMiddleware)
	r.Use(common.CorrelationIDMiddleware)
	r.Use(common.TracingMiddleware("catalog-service"))
	r.Use(common.CORSMiddleware)

	// Register routes
	RegisterRoutes(r)

	// Start server
	port := getPort()
	log.Printf("Product Catalog Service starting on port %d", port)

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
			log.Printf("Failed to parse PORT env var '%s': %v, using default", port, err)
			return 8080
		}
		return p
	}
	return 8080
}
