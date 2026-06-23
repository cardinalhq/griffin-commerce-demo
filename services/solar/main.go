// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

// Package solar simulates the Adani Khavda solar farm demo telemetry
// per docs/specs/adani-solar-farm-simulator.md. It emits OTLP metrics
// for the spec §4 entity catalog (6 blocks → 4 MV transformers in 3
// compounds → 24 inverter stations → 96 inverters → 12 met stations
// → 6 trackers, plus 3 PPAs and a substation) and OTLP logs for the
// spec §16–21 event types. A local HTTP server (/faults/*) lets a demo
// operator flip between baseline and one of the four §29 failure
// profiles.
package solar

import (
	"fmt"
	"log/slog"

	"github.com/cardinalhq/griffin-commerce-demo/common"
)

// Start is the entrypoint invoked by cmd/solar.go. Blocks until the
// telemetry context is cancelled (Ctrl-C / SIGTERM).
func Start() error {
	ctx, shutdown, err := common.SetupTelemetry("solar-service", nil)
	if err != nil {
		return fmt.Errorf("initialize telemetry: %w", err)
	}
	defer func() {
		if err := shutdown(); err != nil {
			slog.Error("Failed to shutdown telemetry", "error", err)
		}
	}()

	catalog := NewCatalog()
	scenario := NewScenario()
	slog.InfoContext(ctx, "Adani solar telemetry simulator catalog built",
		"site", catalog.Site,
		"offtakers", len(catalog.Offtakers),
		"blocks", len(catalog.Blocks),
		"stations", len(catalog.Stations),
		"inverters", len(catalog.Inverters),
		"transformers", len(catalog.Transformers),
		"trackers", len(catalog.Trackers),
		"met_stations", len(catalog.MetStations),
		"profiles", scenario.ProfileIDs(),
	)

	if err := RegisterMetrics(ctx, catalog, scenario); err != nil {
		return fmt.Errorf("register metrics: %w", err)
	}

	StartLogEmitter(ctx, catalog, scenario)
	StartHTTPServer(ctx, scenario)

	slog.InfoContext(ctx, "Adani solar telemetry simulator running. Waiting for shutdown signal.")
	<-ctx.Done()
	slog.InfoContext(ctx, "Adani solar telemetry simulator shutting down")
	return nil
}
