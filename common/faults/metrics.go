// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package faults

import (
	"context"
	"log/slog"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var (
	metricsOnce  sync.Once
	injected     metric.Int64Counter
	addedLatency metric.Float64Histogram
)

func ensureMetrics() {
	metricsOnce.Do(func() {
		meter := otel.Meter("github.com/cardinalhq/griffin-commerce-demo/common/faults")
		var err error
		injected, err = meter.Int64Counter(
			"griffin.faults.injected",
			metric.WithDescription("Number of times a fault knob fired, partitioned by knob key, kind, and service"),
		)
		if err != nil {
			slog.Error("faults: failed to create injected counter", "error", err)
		}
		addedLatency, err = meter.Float64Histogram(
			"griffin.faults.added_latency_ms",
			metric.WithDescription("Additional latency in milliseconds injected by a fault knob"),
			metric.WithUnit("ms"),
		)
		if err != nil {
			slog.Error("faults: failed to create added_latency_ms histogram", "error", err)
		}
	})
}

// Record reports a single fault firing: increments the injected counter,
// records added latency (if positive), and decorates the active span with
// griffin.fault attribute + griffin.fault.fired event so trace UIs see
// fault-fired events on the span timeline even without log correlation.
//
// addedLatencyMs is the *additional* latency the knob caused this fire
// (not the total request latency); pass 0 for error/cpuburn-bg knobs.
func Record(ctx context.Context, k *Knob, addedLatencyMs float64) {
	if k == nil {
		return
	}
	ensureMetrics()

	attrs := []attribute.KeyValue{
		attribute.String("key", k.Key),
		attribute.String("kind", k.Kind),
		attribute.String("service", k.Service),
	}
	if injected != nil {
		injected.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
	if addedLatency != nil && addedLatencyMs > 0 {
		addedLatency.Record(ctx, addedLatencyMs,
			metric.WithAttributes(attribute.String("key", k.Key)))
	}
	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		span.SetAttributes(attribute.String("griffin.fault", k.Key))
		span.AddEvent("griffin.fault.fired", trace.WithAttributes(attrs...))
	}
}
