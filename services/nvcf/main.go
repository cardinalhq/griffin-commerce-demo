// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

// Package nvcf simulates the NVIDIA Cloud Functions demo telemetry per
// docs/specs/nvcf.md. It emits OTLP metrics with the verbatim NVCF native
// metric names and label vocabulary for a synthesized fleet of functions
// × versions × accounts × clusters. A local HTTP server (/faults/*) lets
// a demo operator activate any of the 11 chaos knobs to bend a specific
// cohort's signal — the matching Cardinal dashboard panel lights up.
package nvcf

import (
	"fmt"
	"log/slog"

	"github.com/cardinalhq/griffin-commerce-demo/common"
)

// Start is the entrypoint invoked by cmd/nvcf.go. Blocks until ctx is
// cancelled (Ctrl-C / SIGTERM).
func Start() error {
	ctx, shutdown, err := common.SetupTelemetry("nvcf-service", nil)
	if err != nil {
		return fmt.Errorf("initialize telemetry: %w", err)
	}
	defer func() {
		if err := shutdown(); err != nil {
			slog.Error("Failed to shutdown telemetry", "error", err)
		}
	}()

	catalog := NewCatalog()
	scenario := NewScenario(catalog)
	slog.InfoContext(ctx, "NVCF simulator catalog built",
		"functions", len(catalog.Functions),
		"versions", len(catalog.Versions),
		"accounts", len(catalog.Accounts),
		"clusters", len(catalog.Clusters),
		"instances", len(catalog.Instances),
		"inference_servers", len(catalog.InferenceServers),
		"profiles", scenario.ProfileIDs(),
	)

	if err := RegisterMetrics(ctx, catalog, scenario); err != nil {
		return fmt.Errorf("register metrics: %w", err)
	}

	StartHTTPServer(ctx, scenario)

	slog.InfoContext(ctx, "NVCF simulator running. Waiting for shutdown signal.")
	<-ctx.Done()
	slog.InfoContext(ctx, "NVCF simulator shutting down")
	return nil
}
