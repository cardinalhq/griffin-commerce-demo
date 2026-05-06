// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package payment

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
// payment's processor. Initialized in Start().
var faultsClient *faults.Client

func Start() error {
	// Initialize telemetry with context
	ctx, shutdown, err := common.SetupTelemetry("payment-service", nil)
	if err != nil {
		return fmt.Errorf("failed to initialize telemetry: %w", err)
	}
	defer func() {
		if err := shutdown(); err != nil {
			slog.Error("Failed to shutdown telemetry", "error", err)
		}
	}()

	// Load processor configuration
	if err := LoadProcessorConfig("config.yaml"); err != nil {
		return fmt.Errorf("failed to load processor config: %w", err)
	}
	slog.Info("Payment processors loaded", "count", len(processors))

	// Initialize transaction storage
	InitTransactionStorage()

	// Wire fault-injection client.
	cpuBurn := faults.NewCPUBurnController()
	gcStorm := newGCStormController()
	faultsClient = faults.NewClient(faults.ClientOpts{
		URL:     os.Getenv("CONTROLPLANE_URL"),
		Service: faults.ServicePayment,
		OnActivate: func(ctx context.Context, k *faults.Knob) {
			switch k.Key {
			case "global.cpu-burn-bg":
				cpuBurn.Start(ctx, k)
			case "payment.gc-storm":
				gcStorm.Start(ctx, k.LatencyMs)
			}
		},
		OnClear: func(ctx context.Context, k *faults.Knob) {
			switch k.Key {
			case "global.cpu-burn-bg":
				cpuBurn.Stop(ctx)
			case "payment.gc-storm":
				gcStorm.Stop(ctx)
			}
		},
	})
	faultsClient.Start(ctx)

	// Create router
	r := mux.NewRouter()

	// Apply middleware. TracingMiddleware outermost so LoggingMiddleware
	// sees the otelhttp span context (gives HTTP Request log trace_id).
	r.Use(common.TracingMiddleware("payment-service"))
	r.Use(common.LoggingMiddleware)
	r.Use(common.CorrelationIDMiddleware)
	r.Use(common.CORSMiddleware)
	r.Use(faults.Middleware(faultsClient))

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
		slog.Info("Payment Service starting", "port", port, "processors", []string{"PuppyPay", "KittyCard", "DoggieCoin"})
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
			return 8081
		}
		return p
	}
	return 8081
}