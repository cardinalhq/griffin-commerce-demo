// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package orderevents

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const instrumentScope = "github.com/cardinalhq/griffin-commerce-demo/services/order-events-analyzer"

const (
	topic         = "order.created"
	consumerGroup = "order-events-analyzer"
)

// seed is fixed (not per-entity) — there's exactly one logical consumer
// group being simulated here, unlike dbaas's per-tenant/VM/host fleet.
const seed uint64 = 0x0e5e17

var (
	baselineLag   = Range{2, 6}
	incidentLag   = Range{180, 260}
	baselineDepth = Range{5, 40}
	incidentDepth = Range{4000, 9000}
)

// RegisterMetrics builds the two gauges and registers a single async
// callback that samples the current ramp state on the SDK's collection
// cadence.
func RegisterMetrics(ctx context.Context) error {
	meter := otel.Meter(instrumentScope)

	lag, err := meter.Float64ObservableGauge(
		"griffin.order_events.consumer_lag_seconds",
		metric.WithDescription("Consumer lag for the order.created event stream"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return fmt.Errorf("create consumer_lag_seconds gauge: %w", err)
	}

	depth, err := meter.Float64ObservableGauge(
		"griffin.order_events.backlog_depth",
		metric.WithDescription("Unprocessed message count backlog for the order.created event stream"),
	)
	if err != nil {
		return fmt.Errorf("create backlog_depth gauge: %w", err)
	}

	attrs := metric.WithAttributes(
		attribute.String("topic", topic),
		attribute.String("consumer_group", consumerGroup),
	)

	_, err = meter.RegisterCallback(
		func(_ context.Context, o metric.Observer) error {
			now := time.Now()
			o.ObserveFloat64(lag, RampedValue(baselineLag, incidentLag, seed, now), attrs)
			o.ObserveFloat64(depth, RampedValue(baselineDepth, incidentDepth, seed^1, now), attrs)
			return nil
		},
		lag, depth,
	)
	if err != nil {
		return fmt.Errorf("register order-events callback: %w", err)
	}

	slog.InfoContext(ctx, "order-events-analyzer metrics registered",
		"topic", topic, "consumer_group", consumerGroup, "cycle", cyclePeriod.String())
	return nil
}
