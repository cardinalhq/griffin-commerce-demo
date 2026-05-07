// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package images

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/cardinalhq/griffin-commerce-demo/common"
	"github.com/cardinalhq/griffin-commerce-demo/common/faults"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// faultsClient is the package-level fault-injection polling client.
var faultsClient *faults.Client

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

func Start() error {
	// Initialize telemetry with context
	ctx, shutdown, err := common.SetupTelemetry("image-service", nil)
	if err != nil {
		return fmt.Errorf("failed to initialize telemetry: %w", err)
	}
	defer func() {
		if err := shutdown(); err != nil {
			slog.Error("Failed to shutdown telemetry", "error", err)
		}
	}()

	// Compute hashes for all images on startup
	computeImageHashes()

	// Wire fault-injection client.
	cpuBurn := faults.NewCPUBurnController()
	faultsClient = faults.NewClient(faults.ClientOpts{
		URL:     os.Getenv("CONTROLPLANE_URL"),
		Service: faults.ServiceImages,
		OnActivate: func(ctx context.Context, k *faults.Knob) {
			if k.Key == "global.cpu-burn-bg" {
				cpuBurn.Start(ctx, k)
			}
		},
		OnClear: func(ctx context.Context, k *faults.Knob) {
			if k.Key == "global.cpu-burn-bg" {
				cpuBurn.Stop(ctx)
			}
		},
	})
	faultsClient.Start(ctx)

	// Create router
	r := mux.NewRouter()

	// Apply middleware. TracingMiddleware outermost so LoggingMiddleware
	// sees the otelhttp span context (gives HTTP Request log trace_id).
	r.Use(common.TracingMiddleware("image-service"))
	r.Use(common.LoggingMiddleware)
	r.Use(common.CorrelationIDMiddleware)
	r.Use(common.CORSMiddleware)
	r.Use(faults.Middleware(faultsClient))
	// images.slow + Cache-Control: no-store while active so browser
	// reloads always hit the slow path. Applies to both the API mapping
	// endpoint and /static/* file serves.
	r.Use(imagesSlowMiddleware(faultsClient))

	// Health check
	r.HandleFunc("/health", HealthHandler).Methods("GET")

	// Product image mapping endpoint
	r.HandleFunc("/api/images/product/{id}", GetProductImageHandler).Methods("GET", "OPTIONS")

	// Static file server for /static path
	staticDir := "./services/images/static"
	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		slog.Warn("Static directory does not exist, creating it", "dir", staticDir)
		if err := os.MkdirAll(staticDir, 0755); err != nil {
			slog.Error("Failed to create static directory", "error", err)
		}
	}

	// Serve static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))

	// Create HTTP server
	port := getPort()
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: r,
	}

	// Start server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		slog.Info("Image Service starting", "port", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("server failed: %w", err)
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		slog.Info("Shutting down server due to context cancellation")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown failed: %w", err)
		}
		slog.Info("Server shutdown complete")
		return nil
	case err := <-serverErr:
		return err
	}
}

func getPort() int {
	if port := os.Getenv("PORT"); port != "" {
		var p int
		if _, err := fmt.Sscanf(port, "%d", &p); err != nil {
			slog.Warn("Failed to parse PORT env var, using default", "port", port, "error", err)
			return 8083
		}
		return p
	}
	return 8083
}

// imagesSlowMiddleware sleeps for k.LatencyMs when images.slow is active
// AND sets Cache-Control: no-store on every response so browser reloads
// always hit the slow path. This is the demo-required fidelity detail —
// without no-store, browser cache masks the slow knob.
func imagesSlowMiddleware(c *faults.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if k, fired := faults.Probe(c, "images.slow"); fired && k.LatencyMs > 0 {
				time.Sleep(time.Duration(k.LatencyMs) * time.Millisecond)
				faults.Record(r.Context(), k, float64(k.LatencyMs))
				w.Header().Set("Cache-Control", "no-store")
			}
			next.ServeHTTP(w, r)
		})
	}
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
		slog.ErrorContext(r.Context(), "Failed to write response", "error", err)
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

	trace.SpanFromContext(r.Context()).SetAttributes(
		attribute.String("product.id", productID),
		attribute.String("image.filename", imageInfo.Filename),
		attribute.Bool("image.fallback", !exists),
	)
	slog.InfoContext(r.Context(), "image served",
		"product_id", productID,
		"filename", imageInfo.Filename,
		"hash", response.Hash,
		"fallback", !exists,
	)

	if err := common.WriteJSONResponse(w, response, http.StatusOK); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write response", "error", err)
	}
}

// computeImageHashes computes MD5 hashes for all images in the static directory
func computeImageHashes() {
	staticDir := "./services/images/static"

	files, err := filepath.Glob(filepath.Join(staticDir, "*"))
	if err != nil {
		slog.Error("Failed to list files in static directory", "error", err)
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
			slog.Error("Failed to open file", "file", fileName, "error", err)
			continue
		}

		hash := md5.New()
		if _, err := io.Copy(hash, file); err != nil {
			file.Close()
			slog.Error("Failed to compute hash", "file", fileName, "error", err)
			continue
		}
		file.Close()

		hashString := fmt.Sprintf("%x", hash.Sum(nil))
		imageHashes[fileName] = hashString[:8] // Use first 8 chars for brevity
		slog.Debug("Computed hash for image", "file", fileName, "hash", imageHashes[fileName])
	}
}