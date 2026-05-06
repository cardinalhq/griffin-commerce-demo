// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package catalog

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/cardinalhq/griffin-commerce-demo/common"
	"github.com/cardinalhq/griffin-commerce-demo/common/faults"
	"github.com/gorilla/mux"
)

// faultsClient is the package-level fault-injection polling client used by
// catalog's request handlers. Initialized in Start().
var faultsClient *faults.Client

func Start() error {
	// Initialize telemetry with context
	ctx, shutdown, err := common.SetupTelemetry("catalog-service", nil)
	if err != nil {
		return fmt.Errorf("failed to initialize telemetry: %w", err)
	}
	defer func() {
		if err := shutdown(); err != nil {
			slog.Error("Failed to shutdown telemetry", "error", err)
		}
	}()

	// Load products from YAML
	if err := LoadProducts("products.yaml"); err != nil {
		return fmt.Errorf("failed to load products: %w", err)
	}
	slog.Info("Products loaded", "count", GetProductCount())

	// Wire fault-injection client. Polls the control plane every ~1s and
	// exposes the active knob via faultsClient.Active() to handlers. The
	// background CPU-burn controller runs in every service so the
	// global.cpu-burn-bg knob saturates every container equally.
	cpuBurn := faults.NewCPUBurnController()
	faultsClient = faults.NewClient(faults.ClientOpts{
		URL:     os.Getenv("CONTROLPLANE_URL"),
		Service: faults.ServiceCatalog,
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
	r.Use(common.TracingMiddleware("catalog-service"))
	r.Use(common.LoggingMiddleware)
	r.Use(common.CorrelationIDMiddleware)
	r.Use(common.CORSMiddleware)
	// Generic middleware handles global.cpu-burn-traffic and any global
	// slow/error knobs. Service-specific knobs are handled at handler
	// level so per-product / per-operation labels land on the metrics.
	r.Use(faults.Middleware(faultsClient))
	// catalog.slow lives at middleware level — no per-handler labels needed.
	r.Use(faults.SlowMiddleware(faultsClient, "catalog.slow"))

	// Register routes
	RegisterRoutes(r)

	// Create HTTP server
	port := getPort()
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: r,
	}

	// Start server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		slog.Info("Product Catalog Service starting", "port", port)
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
			return 8080
		}
		return p
	}
	return 8080
}