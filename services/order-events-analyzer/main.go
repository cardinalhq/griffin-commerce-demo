// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package orderevents

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

// Start is the entrypoint invoked by cmd/order-events-analyzer.go. It has
// no admin/fault-injection surface — the lag cycle is fully self-triggering
// off wall-clock time — so the HTTP server here only exists for the k8s
// liveness/readiness probes every other service also exposes. Blocks until
// the telemetry context is cancelled (Ctrl-C / SIGTERM).
func Start() error {
	ctx, shutdown, err := common.SetupTelemetry("order-events-analyzer-service", nil)
	if err != nil {
		return fmt.Errorf("failed to initialize telemetry: %w", err)
	}
	defer func() {
		if err := shutdown(); err != nil {
			slog.Error("Failed to shutdown telemetry", "error", err)
		}
	}()

	if err := RegisterMetrics(ctx); err != nil {
		return fmt.Errorf("register metrics: %w", err)
	}
	StartLogEmitter(ctx)

	r := mux.NewRouter()
	r.Use(common.TracingMiddleware("order-events-analyzer-service"))
	r.Use(common.LoggingMiddleware)
	r.Use(common.CorrelationIDMiddleware)
	r.Use(common.CORSMiddleware)
	r.HandleFunc("/health", healthHandler).Methods("GET")

	port := getPort()
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("order-events-analyzer starting", "port", port, "cycle", cyclePeriod.String())
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("server failed: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		slog.Info("Shutting down order-events-analyzer")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown failed: %w", err)
		}
		return nil
	case err := <-serverErr:
		return err
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if err := common.WriteJSONResponse(w, common.HealthResponse{
		Status:    "healthy",
		Service:   "order-events-analyzer-service",
		Version:   "1.0.0",
		Timestamp: time.Now(),
	}, http.StatusOK); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write health response", "error", err)
	}
}

func getPort() int {
	if port := os.Getenv("PORT"); port != "" {
		var p int
		if _, err := fmt.Sscanf(port, "%d", &p); err == nil {
			return p
		}
	}
	return 8087
}
