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
	"github.com/cardinalhq/griffin-commerce-demo/common/faults"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// faultsClient is the package-level fault-injection polling client.
var faultsClient *faults.Client

// defaultRefreshInterval is the cadence at which the recs cache reloads
// from catalog. The plan reduces this from the original 5 minutes to 30
// seconds so the catalog.error → recs cascade is visible promptly.
const defaultRefreshInterval = 30 * time.Second

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
		Timeout: 10 * time.Second,
		Transport: otelhttp.NewTransport(
			http.DefaultTransport,
			otelhttp.WithSpanOptions(trace.WithAttributes(attribute.String("peer.service", "catalog"))),
		),
	}

	// Load initial products
	if err := loadProductsFromCatalog(ctx); err != nil {
		slog.WarnContext(ctx, "Failed to preload products", "error", err)
	}

	// Wire fault-injection client.
	cpuBurn := faults.NewCPUBurnController()
	memLeak := newMemLeakController()
	faultsClient = faults.NewClient(faults.ClientOpts{
		URL:     os.Getenv("CONTROLPLANE_URL"),
		Service: faults.ServiceRecommendations,
		OnActivate: func(ctx context.Context, k *faults.Knob) {
			switch k.Key {
			case "global.cpu-burn-bg":
				cpuBurn.Start(ctx, k)
			case "recs.memleak":
				memLeak.Start(ctx)
			}
		},
		OnClear: func(ctx context.Context, k *faults.Knob) {
			switch k.Key {
			case "global.cpu-burn-bg":
				cpuBurn.Stop(ctx)
			case "recs.memleak":
				memLeak.Stop(ctx)
			}
		},
	})
	faultsClient.Start(ctx)

	// Background refresh — interval is env-configurable so the demo can
	// shorten it (default 30s; set RECS_REFRESH_INTERVAL=5m to restore the
	// historical cadence).
	refreshDone = make(chan struct{})
	refreshTicker = time.NewTicker(refreshInterval())
	go func() {
		defer refreshTicker.Stop()
		for {
			select {
			case <-refreshTicker.C:
				if err := loadProductsFromCatalog(ctx); err != nil {
					slog.ErrorContext(ctx, "Failed to refresh product cache", "error", err)
				}
			case <-refreshDone:
				return
			}
		}
	}()

	// Create router
	r := mux.NewRouter()

	// Apply middleware. TracingMiddleware outermost so LoggingMiddleware
	// sees the otelhttp span context (gives HTTP Request log trace_id).
	r.Use(common.TracingMiddleware("recommendations-service"))
	r.Use(common.LoggingMiddleware)
	r.Use(common.CorrelationIDMiddleware)
	r.Use(common.CORSMiddleware)
	r.Use(faults.Middleware(faultsClient))

	// Register routes
	r.HandleFunc("/health", HealthHandler).Methods("GET")
	r.HandleFunc("/api/recommendations", GetRecommendationsHandler).Methods("GET")
	r.HandleFunc("/api/recommendations/product/{id}", GetProductRecommendationsHandler).Methods("GET")
	// Admin route used to force a cache refresh — useful for the
	// catalog.error cascade demo so investigators don't wait for the next
	// ticker tick to see recs picking up the upstream failure.
	r.HandleFunc("/admin/recs/refresh", AdminRefreshHandler(ctx)).Methods("POST")

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

// refreshInterval reads RECS_REFRESH_INTERVAL (e.g. "30s", "5m"); falls
// back to the demo default when unset or unparseable.
func refreshInterval() time.Duration {
	if v := os.Getenv("RECS_REFRESH_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
		slog.Warn("Failed to parse RECS_REFRESH_INTERVAL, using default", "value", v)
	}
	return defaultRefreshInterval
}

// AdminRefreshHandler returns a handler that triggers an immediate cache
// reload from catalog. Used by demos that need the catalog.error cascade
// to surface in recs without waiting for the next ticker tick.
func AdminRefreshHandler(rootCtx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Use the request context so the refresh is tied to the trace,
		// but fall back to the long-lived rootCtx if the request ctx is
		// already done.
		ctx := r.Context()
		if ctx.Err() != nil {
			ctx = rootCtx
		}
		if err := loadProductsFromCatalog(ctx); err != nil {
			slog.ErrorContext(r.Context(), "admin refresh failed", "error", err)
			correlationID := common.GetCorrelationID(r.Context())
			common.WriteErrorResponse(r.Context(), w,
				common.NewAppError("REFRESH_FAILED", err.Error()),
				http.StatusInternalServerError, correlationID)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
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
			slog.WarnContext(ctx, "Failed to close response body", "error", err)
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

	slog.InfoContext(ctx, "Loaded products from catalog", "count", len(products))
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
		slog.ErrorContext(r.Context(), "Failed to write response", "error", err)
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

	trace.SpanFromContext(r.Context()).SetAttributes(
		attribute.String("recommendations.type", response.Type),
		attribute.Int("recommendations.requested_count", count),
		attribute.Int("recommendations.returned_count", len(recommendations)),
	)
	slog.InfoContext(r.Context(), "recommendations served",
		"type", response.Type,
		"requested_count", count,
		"returned_count", len(recommendations),
	)

	if err := common.WriteJSONResponse(w, response, http.StatusOK); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write response", "error", err)
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

	trace.SpanFromContext(r.Context()).SetAttributes(
		attribute.String("product.id", productID),
		attribute.String("recommendations.type", response.Type),
		attribute.Int("recommendations.requested_count", count),
		attribute.Int("recommendations.returned_count", len(recommendations)),
	)
	slog.InfoContext(r.Context(), "product recommendations served",
		"product_id", productID,
		"type", response.Type,
		"requested_count", count,
		"returned_count", len(recommendations),
	)

	if err := common.WriteJSONResponse(w, response, http.StatusOK); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write response", "error", err)
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