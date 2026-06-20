// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

// Package dbaas simulates the Airtel PostgreSQL-on-VMware demo telemetry
// per docs/specs/airtel-telemetry-simulator.md. It emits OTLP metrics for
// the spec §4 entity catalog (6 tenants → 4 PG instances → 6 VMs → 4 ESXi
// hosts → 3 datastores) and OTLP logs for the spec §16–21 event types.
// A local HTTP server (/faults/*) lets a demo operator flip between
// baseline and one of the four §29 failure profiles.
package dbaas

import (
	"fmt"
	"log/slog"

	"github.com/cardinalhq/griffin-commerce-demo/common"
)

// Start is the entrypoint invoked by cmd/dbaas.go. Blocks until the
// telemetry context is cancelled (Ctrl-C / SIGTERM).
func Start() error {
	ctx, shutdown, err := common.SetupTelemetry("dbaas-service", nil)
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
	slog.InfoContext(ctx, "Airtel telemetry simulator catalog built",
		"tenants", len(catalog.Tenants),
		"pg_instances", len(catalog.PGInstances),
		"vms", len(catalog.VMs),
		"hosts", len(catalog.Hosts),
		"datastores", len(catalog.Datastores),
		"profiles", scenario.ProfileIDs(),
	)

	if err := RegisterMetrics(ctx, catalog, scenario); err != nil {
		return fmt.Errorf("register metrics: %w", err)
	}

	StartLogEmitter(ctx, catalog, scenario)
	StartHTTPServer(ctx, scenario)

	slog.InfoContext(ctx, "Airtel telemetry simulator running. Waiting for shutdown signal.")
	<-ctx.Done()
	slog.InfoContext(ctx, "Airtel telemetry simulator shutting down")
	return nil
}
