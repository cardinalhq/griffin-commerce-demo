// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package cart

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
// cart's request handlers. Initialized in Start().
var faultsClient *faults.Client

func Start() error {
	// Initialize telemetry with context
	ctx, shutdown, err := common.SetupTelemetry("cart-service", nil)
	if err != nil {
		return fmt.Errorf("failed to initialize telemetry: %w", err)
	}
	defer func() {
		if err := shutdown(); err != nil {
			slog.Error("Failed to shutdown telemetry", "error", err)
		}
	}()

	// Initialize cart storage
	InitCartStorage()

	// Initialize product client
	catalogURL := os.Getenv("CATALOG_SERVICE_URL")
	if catalogURL == "" {
		catalogURL = "http://localhost:8080"
	}
	InitProductClient(catalogURL)

	// Initialize payment + shipping clients so CheckoutHandler can fan out.
	// Defaults match the k8s Service names in k8s/base/backend-services.yaml.
	paymentURL := os.Getenv("PAYMENT_SERVICE_URL")
	if paymentURL == "" {
		paymentURL = "http://payment:8081"
	}
	InitPaymentClient(paymentURL)

	shippingURL := os.Getenv("SHIPPING_SERVICE_URL")
	if shippingURL == "" {
		shippingURL = "http://shipping:8083"
	}
	InitShippingClient(shippingURL)

	// Wire fault-injection client.
	cpuBurn := faults.NewCPUBurnController()
	faultsClient = faults.NewClient(faults.ClientOpts{
		URL:     os.Getenv("CONTROLPLANE_URL"),
		Service: faults.ServiceCart,
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

	// Apply middleware. TracingMiddleware must be outermost so that
	// LoggingMiddleware sees a request whose context already carries the
	// otelhttp span — that's how the HTTP Request log line acquires
	// trace_id/span_id.
	r.Use(common.TracingMiddleware("cart-service"))
	r.Use(common.LoggingMiddleware)
	r.Use(common.CorrelationIDMiddleware)
	r.Use(common.CORSMiddleware)
	// Generic middleware handles global knobs only. cart.error / cart.outlier
	// / cart.poison-product live at handler level so per-operation metric
	// labels still get emitted on the failure path.
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
		slog.Info("Cart Service starting",
			"port", port,
			"catalog_url", catalogURL,
			"payment_url", paymentURL,
			"shipping_url", shippingURL,
		)
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
			return 8082
		}
		return p
	}
	return 8082
}
