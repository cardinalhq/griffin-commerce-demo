// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

// Package controlplane is the source of truth for the single active
// fault-injection knob. Per-service polling clients in common/faults
// fetch state from here every ~1s.
package controlplane

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/cardinalhq/griffin-commerce-demo/common"
	"github.com/gorilla/mux"
)

func Start() error {
	ctx, shutdown, err := common.SetupTelemetry("controlplane-service", nil)
	if err != nil {
		return fmt.Errorf("failed to initialize telemetry: %w", err)
	}
	defer func() {
		if err := shutdown(); err != nil {
			slog.Error("Failed to shutdown telemetry", "error", err)
		}
	}()

	s := newState()

	r := mux.NewRouter()

	// TracingMiddleware outermost — same ordering as the product services
	// so LoggingMiddleware sees the otelhttp span context.
	r.Use(common.TracingMiddleware("controlplane-service"))
	r.Use(common.LoggingMiddleware)
	r.Use(common.CorrelationIDMiddleware)
	r.Use(common.CORSMiddleware)

	RegisterRoutes(r, s)

	port := getPort()
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: r,
		// SSE streams are long-lived; don't impose a write timeout that
		// would cut the stream mid-flight.
		ReadHeaderTimeout: 10 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("Control Plane Service starting",
			"port", port,
			"admin_enabled", adminEnabled(),
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("server failed: %w", err)
		}
	}()

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
			return 8086
		}
		return p
	}
	return 8086
}
