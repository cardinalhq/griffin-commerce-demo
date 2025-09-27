package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/griffincommerce/demo/common"
)

// ImageInfo contains image details
type ImageInfo struct {
	Filename string
	Offset   string // "top", "center", or "bottom" for object-position
}

// Product to image mapping with offset information
var productImages = map[string]ImageInfo{
	"PROD-001": {Filename: "dog-food.jpg", Offset: "center"},     // Dog food - keep centered
	"PROD-002": {Filename: "rope-toy.jpg", Offset: "30% 50%"},    // Rope toy - show dogs with toy (30% from top)
	"PROD-003": {Filename: "dog-bed.jpg", Offset: "20% 50%"},     // Dog bed - show Mocha's head fully (20% from top)
	"PROD-004": {Filename: "collar.jpg", Offset: "top"},          // Collar - show upper part with dog's face
	"PROD-005": {Filename: "tennis-balls.jpg", Offset: "center"},  // Tennis balls - keep centered
	"PROD-006": {Filename: "shampoo.jpg", Offset: "15% 50%"},     // Shampoo - show more of dog's face (15% from top)
}

// Image file hashes for cache-busting
var imageHashes = map[string]string{}

func main() {
	// Initialize telemetry
	shutdown, err := common.InitTelemetry("image-service")
	if err != nil {
		log.Printf("Failed to initialize telemetry: %v", err)
	}
	defer shutdown()

	// Compute hashes for all images on startup
	computeImageHashes()

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
	r.HandleFunc("/api/images/product/{id}", GetProductImageHandler).Methods("GET", "OPTIONS")

	// Static file server for /static path
	staticDir := "./services/images/static"
	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		log.Printf("Warning: static directory does not exist, creating it")
		if err := os.MkdirAll(staticDir, 0755); err != nil {
			log.Printf("Failed to create static directory: %v", err)
		}
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
		if _, err := fmt.Sscanf(port, "%d", &p); err != nil {
			log.Printf("Failed to parse port: %v", err)
			return 8083
		}
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
	if err := common.WriteJSONResponse(w, health, http.StatusOK); err != nil {
		slog.Error("Failed to write response", "error", err)
	}
}

// ImageResponse represents an image URL response
type ImageResponse struct {
	ProductID string `json:"product_id"`
	ImageURL  string `json:"image_url"`
	Hash      string `json:"hash,omitempty"`
	Offset    string `json:"offset,omitempty"`
}

// GetProductImageHandler returns the image URL for a product
func GetProductImageHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productID := vars["id"]

	imageInfo, exists := productImages[productID]
	if !exists {
		// Return a default image if product not found
		imageInfo = ImageInfo{Filename: "placeholder.jpg", Offset: "center"}
	}

	response := ImageResponse{
		ProductID: productID,
		ImageURL:  fmt.Sprintf("/static/%s", imageInfo.Filename),
		Hash:      imageHashes[imageInfo.Filename],
		Offset:    imageInfo.Offset,
	}

	if err := common.WriteJSONResponse(w, response, http.StatusOK); err != nil {
		slog.Error("Failed to write response", "error", err)
	}
}

// computeImageHashes computes MD5 hashes for all images in the static directory
func computeImageHashes() {
	staticDir := "./services/images/static"

	files, err := filepath.Glob(filepath.Join(staticDir, "*"))
	if err != nil {
		log.Printf("Failed to list files in static directory: %v", err)
		return
	}

	for _, filePath := range files {
		fileName := filepath.Base(filePath)

		// Skip directories and non-image files
		info, err := os.Stat(filePath)
		if err != nil || info.IsDir() {
			continue
		}

		// Compute MD5 hash
		file, err := os.Open(filePath)
		if err != nil {
			log.Printf("Failed to open file %s: %v", fileName, err)
			continue
		}

		hash := md5.New()
		if _, err := io.Copy(hash, file); err != nil {
			file.Close()
			log.Printf("Failed to compute hash for %s: %v", fileName, err)
			continue
		}
		file.Close()

		hashString := fmt.Sprintf("%x", hash.Sum(nil))
		imageHashes[fileName] = hashString[:8] // Use first 8 chars for brevity
		log.Printf("Computed hash for %s: %s", fileName, imageHashes[fileName])
	}
}
