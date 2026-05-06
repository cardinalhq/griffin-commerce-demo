// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package faults

import (
	"context"
	"log/slog"
	"strconv"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Site metrics. These are emitted unconditionally by every request — they're
// not fault-injection metrics, they're product metrics with the right label
// cardinality so Conductor's detect_outliers tool can locate cohorts at the
// product / processor / carrier / operation level. Without them the demo's
// outlier detection stories don't have anywhere to land.

var (
	siteMetricsOnce sync.Once

	catalogProductRequests metric.Int64Counter
	catalogProductDuration metric.Float64Histogram

	cartOpRequests metric.Int64Counter
	cartOpDuration metric.Float64Histogram

	paymentCharges        metric.Int64Counter
	paymentChargeDuration metric.Float64Histogram

	shippingShipments metric.Int64Counter
)

func ensureSiteMetrics() {
	siteMetricsOnce.Do(func() {
		meter := otel.Meter("github.com/cardinalhq/griffin-commerce-demo")

		var err error
		catalogProductRequests, err = meter.Int64Counter(
			"griffin.catalog.product.requests_total",
			metric.WithDescription("Catalog product GET requests partitioned by product_id and HTTP status"),
		)
		if err != nil {
			slog.Error("create catalog requests counter", "error", err)
		}
		catalogProductDuration, err = meter.Float64Histogram(
			"griffin.catalog.product.duration_ms",
			metric.WithDescription("Catalog product GET request duration in milliseconds, by product_id"),
			metric.WithUnit("ms"),
		)
		if err != nil {
			slog.Error("create catalog duration histogram", "error", err)
		}

		cartOpRequests, err = meter.Int64Counter(
			"griffin.cart.operations_total",
			metric.WithDescription("Cart operations partitioned by operation and HTTP status"),
		)
		if err != nil {
			slog.Error("create cart op counter", "error", err)
		}
		cartOpDuration, err = meter.Float64Histogram(
			"griffin.cart.operation.duration_ms",
			metric.WithDescription("Cart operation duration in milliseconds, by operation"),
			metric.WithUnit("ms"),
		)
		if err != nil {
			slog.Error("create cart op duration histogram", "error", err)
		}

		paymentCharges, err = meter.Int64Counter(
			"griffin.payment.charges_total",
			metric.WithDescription("Payment charges partitioned by processor and status"),
		)
		if err != nil {
			slog.Error("create payment charges counter", "error", err)
		}
		paymentChargeDuration, err = meter.Float64Histogram(
			"griffin.payment.charge.duration_ms",
			metric.WithDescription("Payment charge processing duration in milliseconds, by processor"),
			metric.WithUnit("ms"),
		)
		if err != nil {
			slog.Error("create payment charge duration histogram", "error", err)
		}

		shippingShipments, err = meter.Int64Counter(
			"griffin.shipping.shipments_total",
			metric.WithDescription("Shipping attempts partitioned by carrier and status"),
		)
		if err != nil {
			slog.Error("create shipping shipments counter", "error", err)
		}
	})
}

// RecordCatalogProduct records one GET /api/products/{id} request.
func RecordCatalogProduct(ctx context.Context, productID string, statusCode int, durationMs float64) {
	ensureSiteMetrics()
	if catalogProductRequests != nil {
		catalogProductRequests.Add(ctx, 1, metric.WithAttributes(
			attribute.String("product_id", productID),
			attribute.String("http_status_code", strconv.Itoa(statusCode)),
		))
	}
	if catalogProductDuration != nil {
		catalogProductDuration.Record(ctx, durationMs, metric.WithAttributes(
			attribute.String("product_id", productID),
		))
	}
}

// RecordCartOp records one cart operation. operation ∈ {get,add,remove,checkout,clear,create}.
func RecordCartOp(ctx context.Context, operation string, statusCode int, durationMs float64) {
	ensureSiteMetrics()
	if cartOpRequests != nil {
		cartOpRequests.Add(ctx, 1, metric.WithAttributes(
			attribute.String("operation", operation),
			attribute.String("http_status_code", strconv.Itoa(statusCode)),
		))
	}
	if cartOpDuration != nil {
		cartOpDuration.Record(ctx, durationMs, metric.WithAttributes(
			attribute.String("operation", operation),
		))
	}
}

// RecordPaymentCharge records one payment charge. status ∈ {success, failed}.
func RecordPaymentCharge(ctx context.Context, processor, status string, durationMs float64) {
	ensureSiteMetrics()
	if paymentCharges != nil {
		paymentCharges.Add(ctx, 1, metric.WithAttributes(
			attribute.String("processor", processor),
			attribute.String("status", status),
		))
	}
	if paymentChargeDuration != nil {
		paymentChargeDuration.Record(ctx, durationMs, metric.WithAttributes(
			attribute.String("processor", processor),
		))
	}
}

// RecordShipment records one shipping attempt. status ∈ {shipped, failed}.
func RecordShipment(ctx context.Context, carrier, status string) {
	ensureSiteMetrics()
	if shippingShipments != nil {
		shippingShipments.Add(ctx, 1, metric.WithAttributes(
			attribute.String("carrier", carrier),
			attribute.String("status", status),
		))
	}
}
