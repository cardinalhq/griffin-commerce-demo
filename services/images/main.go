package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/griffincommerce/demo/common"
)

// Product to image mapping
var productImages = map[string]string{
	"PROD-001": "rope-toy.jpg",
	"PROD-002": "bacon-treats.jpg",
	"PROD-003": "tennis-balls.jpg",
	"PROD-004": "puppy-food.jpg",
	"PROD-005": "dog-bed.jpg",
	"PROD-006": "dental-chews.jpg",
	"PROD-007": "puzzle-toy.jpg",
	"PROD-008": "pb-treats.jpg",
	"PROD-009": "collar.jpg",
	"PROD-010": "water-bowl.jpg",
}

func main() {
	// Initialize telemetry
	shutdown, err := common.InitTelemetry("image-service")
	if err != nil {
		log.Printf("Failed to initialize telemetry: %v", err)
	}
	defer shutdown()

	// Create router
	r := mux.NewRouter()

	// Apply middleware
	r.Use(common.LoggingMiddleware)
	r.Use(common.CorrelationIDMiddleware)
	r.Use(common.TracingMiddleware("image-service"))
	r.Use(common.CORSMiddleware)

	// Health check
	r.HandleFunc("/health", HealthHandler).Methods("GET")

	// Product image mapping endpoint
	r.HandleFunc("/api/images/product/{id}", GetProductImageHandler).Methods("GET")

	// Static file server for /static path
	staticDir := "./static"
	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		log.Printf("Warning: static directory does not exist, creating it")
		os.MkdirAll(staticDir, 0755)
	}

	// Serve static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))

	// Start server
	port := getPort()
	log.Printf("Image Service starting on port %d", port)

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
	return 8083
}

// HealthHandler returns service health status
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	health := common.HealthResponse{
		Status:    "healthy",
		Service:   "image-service",
		Version:   "1.0.0",
		Timestamp: time.Now(),
	}
	common.WriteJSONResponse(w, health, http.StatusOK)
}

// ImageResponse represents an image URL response
type ImageResponse struct {
	ProductID string `json:"product_id"`
	ImageURL  string `json:"image_url"`
}

// GetProductImageHandler returns the image URL for a product
func GetProductImageHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productID := vars["id"]

	imageName, exists := productImages[productID]
	if !exists {
		// Return a default image if product not found
		imageName = "placeholder.jpg"
	}

	response := ImageResponse{
		ProductID: productID,
		ImageURL:  fmt.Sprintf("/static/%s", imageName),
	}

	common.WriteJSONResponse(w, response, http.StatusOK)
}
