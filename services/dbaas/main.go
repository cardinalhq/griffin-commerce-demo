// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

// Package dbaas simulates a multi-tenant managed-Postgres fleet for the
// Airtel DBaaS demo. Unlike the commerce services it doesn't serve HTTP
// requests — it just emits OTLP metrics for every DB instance in the fleet
// on the SDK's collection cadence. See docs/plans/airtel-demo-plan.md in
// the conductor repo for the demo scenario this powers.
package dbaas

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/cardinalhq/griffin-commerce-demo/common"
)

// Start is the entrypoint invoked by cmd/dbaas.go. It blocks until the
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

	fleetState = buildFleetState(time.Now())
	slog.InfoContext(ctx, "DBaaS fleet built",
		"customers", len(Fleet),
		"instances", len(fleetState),
	)

	if err := RegisterMetrics(ctx, fleetState); err != nil {
		return fmt.Errorf("register metrics: %w", err)
	}

	// Wire the controlplane polling client. Demo presenter activates
	// dbaas.disk-full via the controlplane UI to trigger the scenario.
	startFaultsClient(ctx)

	slog.InfoContext(ctx, "DBaaS simulator running. Waiting for shutdown signal.")
	<-ctx.Done()
	slog.InfoContext(ctx, "DBaaS simulator shutting down")
	return nil
}
