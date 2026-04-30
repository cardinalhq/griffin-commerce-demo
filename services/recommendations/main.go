// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package recommendations

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/cardinalhq/griffin-commerce-demo/common"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

var (
	productCache      []common.Product
	productCacheMutex sync.RWMutex
	catalogClient     *http.Client
	refreshTicker     *time.Ticker
	refreshDone       chan struct{}
)

func Start() error {
	// Initialize telemetry with context
	ctx, shutdown, err := common.SetupTelemetry("recommendations-service", nil)
	if err != nil {
		return fmt.Errorf("failed to initialize telemetry: %w", err)
	}
	defer func() {
		if err := shutdown(); err != nil {
			slog.Error("Failed to shutdown telemetry", "error", err)
		}
	}()

	catalogClient = &http.Client{
		Timeout:   10 * time.Second,
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	// Load initial products
	if err := loadProductsFromCatalog(ctx); err != nil {
		slog.Warn("Failed to preload products", "error", err)
	}

	// Start background refresh goroutine
	refreshDone = make(chan struct{})
	refreshTicker = time.NewTicker(5 * time.Minute)
	go func() {
		defer refreshTicker.Stop()
		for {
			select {
			case <-refreshTicker.C:
				if err := loadProductsFromCatalog(ctx); err != nil {
					slog.Error("Failed to refresh product cache", "error", err)
				}
			case <-refreshDone:
				return
			}
		}
	}()

	// Create router
	r := mux.NewRouter()

	// Apply middleware
	r.Use(common.LoggingMiddleware)
	r.Use(common.CorrelationIDMiddleware)
	r.Use(common.TracingMiddleware("recommendations-service"))
	r.Use(common.CORSMiddleware)

	// Register routes
	r.HandleFunc("/health", HealthHandler).Methods("GET")
	r.HandleFunc("/api/recommendations", GetRecommendationsHandler).Methods("GET")
	r.HandleFunc("/api/recommendations/product/{id}", GetProductRecommendationsHandler).Methods("GET")

	// Create HTTP server
	port := getPort()
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: r,
	}

	// Start server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		slog.Info("Recommendations Service starting", "port", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("server failed: %w", err)
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		slog.Info("Shutting down server due to context cancellation")

		// Stop the refresh ticker
		close(refreshDone)

		// Shutdown HTTP server
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown failed: %w", err)
		}
		slog.Info("Server shutdown complete")
		return nil
	case err := <-serverErr:
		close(refreshDone)
		return err
	}
}

func getPort() int {
	if port := os.Getenv("PORT"); port != "" {
		var p int
		if _, err := fmt.Sscanf(port, "%d", &p); err != nil {
			slog.Warn("Failed to parse PORT env var, using default", "port", port, "error", err)
			return 8085
		}
		return p
	}
	return 8085
}

func loadProductsFromCatalog(ctx context.Context) error {
	catalogURL := os.Getenv("CATALOG_SERVICE_URL")
	if catalogURL == "" {
		catalogURL = "http://localhost:8080"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, catalogURL+"/api/products", nil)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	resp, err := catalogClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch products: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Warn("Failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("catalog service returned status %d", resp.StatusCode)
	}

	var products []common.Product

	if err := json.NewDecoder(resp.Body).Decode(&products); err != nil {
		return fmt.Errorf("failed to decode products: %w", err)
	}

	productCacheMutex.Lock()
	productCache = products
	productCacheMutex.Unlock()

	slog.Info("Loaded products from catalog", "count", len(products))
	return nil
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	health := common.HealthResponse{
		Status:    "healthy",
		Service:   "recommendations-service",
		Version:   "1.0.0",
		Timestamp: time.Now(),
	}
	if err := common.WriteJSONResponse(w, health, http.StatusOK); err != nil {
		slog.Error("Failed to write response", "error", err)
	}
}

type RecommendationsResponse struct {
	Products []common.Product `json:"products"`
	Type     string           `json:"type"`
}

func GetRecommendationsHandler(w http.ResponseWriter, r *http.Request) {
	count := 5
	if countStr := r.URL.Query().Get("count"); countStr != "" {
		if c, err := strconv.Atoi(countStr); err == nil && c > 0 && c <= 20 {
			count = c
		}
	}

	recommendations := getRandomRecommendations(count, "")

	response := RecommendationsResponse{
		Products: recommendations,
		Type:     "trending",
	}

	if err := common.WriteJSONResponse(w, response, http.StatusOK); err != nil {
		slog.Error("Failed to write response", "error", err)
	}
}

func GetProductRecommendationsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productID := vars["id"]

	count := 4
	if countStr := r.URL.Query().Get("count"); countStr != "" {
		if c, err := strconv.Atoi(countStr); err == nil && c > 0 && c <= 20 {
			count = c
		}
	}

	recommendations := getRandomRecommendations(count, productID)

	response := RecommendationsResponse{
		Products: recommendations,
		Type:     "related",
	}

	if err := common.WriteJSONResponse(w, response, http.StatusOK); err != nil {
		slog.Error("Failed to write response", "error", err)
	}
}

func getRandomRecommendations(count int, excludeID string) []common.Product {
	productCacheMutex.RLock()
	defer productCacheMutex.RUnlock()

	if len(productCache) == 0 {
		return []common.Product{}
	}

	var eligibleProducts []common.Product
	for _, product := range productCache {
		if product.ID != excludeID && product.Stock > 0 {
			eligibleProducts = append(eligibleProducts, product)
		}
	}

	if len(eligibleProducts) == 0 {
		return []common.Product{}
	}

	rand.Shuffle(len(eligibleProducts), func(i, j int) {
		eligibleProducts[i], eligibleProducts[j] = eligibleProducts[j], eligibleProducts[i]
	})

	if count > len(eligibleProducts) {
		count = len(eligibleProducts)
	}

	return eligibleProducts[:count]
}