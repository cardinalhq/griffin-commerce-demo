// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package payment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// dbDiskFullActive flips when the dbaas.disk-full knob is active. Set by
// the faults.Client callback in main.go; read on the request hot path.
//
// This is the spine of the Airtel cross-persona demo: a single knob fires
// matching telemetry on both subsystems — dbaas service emits the metric
// state, payment service emits the trace + log state. Cleared by clearing
// the knob (the demo's "online volume expansion" resolution).
var dbDiskFullActive atomic.Bool

// dbInstanceID is the DBaaS instance the payment service "talks to." In
// demo mode this is HDFC's hdfc-prod-03. Override via env var if we expand
// to other customers later.
func dbInstanceID() string {
	if v := os.Getenv("PAYMENT_DB_INSTANCE_ID"); v != "" {
		return v
	}
	return "hdfc-prod-03"
}

// errDBDiskFull mirrors the message a real Postgres driver returns for
// SQLSTATE 53100. Wrapped in span / log attributes for the demo.
var errDBDiskFull = errors.New(`pq: could not extend file "base/16384/24779": No space left on device`)

// RecordOrder emits a child span representing the INSERT into the orders
// table on the configured DBaaS instance. Returns an error when the
// disk-full knob is active; otherwise simulates a quick (5–12ms) success.
// This is the load-bearing span for the customer-persona demo: when the
// presenter opens a failed checkout trace in HDFC's AppView, this is the
// red span they drill into.
func RecordOrder(ctx context.Context, orderID string, amount float64) error {
	tracer := otel.Tracer("github.com/cardinalhq/griffin-commerce-demo/services/payment")
	ctx, span := tracer.Start(ctx, "db.write")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.instance.id", dbInstanceID()),
		attribute.String("db.name", "smarthub"),
		attribute.String("db.operation", "INSERT"),
		attribute.String("db.statement", "INSERT INTO orders (order_id, amount) VALUES ($1, $2)"),
	)

	if dbDiskFullActive.Load() {
		time.Sleep(12 * time.Millisecond) // simulate Postgres' try-then-fail latency
		span.SetAttributes(
			attribute.String("error.type", "SQLSTATE"),
			attribute.String("error.code", "53100"),
			attribute.String("error.message", "could not extend file: No space left on device"),
		)
		span.SetStatus(codes.Error, "SQLSTATE 53100: disk full")
		slog.ErrorContext(ctx, "DB write failed",
			"db.instance.id", dbInstanceID(),
			"order_id", orderID,
			"sqlstate", "53100",
			"error", errDBDiskFull.Error(),
		)
		return fmt.Errorf("record order: %w", errDBDiskFull)
	}

	// Normal-path simulated latency. Use math/rand (not crypto) — this
	// isn't security-sensitive, just demo jitter.
	sleep := time.Duration(5+rand.Intn(8)) * time.Millisecond
	time.Sleep(sleep)
	return nil
}
