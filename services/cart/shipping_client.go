// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package cart

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var (
	shippingClient  *http.Client
	shippingBaseURL string
)

// shippingCarriers matches the keys in services/shipping/config.yaml.
// Rotating so per-carrier failure rates (5% / 10% / 25% baseline; the
// shipping.fail knob targets a single one) are exercised.
var shippingCarriers = []string{"ponyexpress", "avianair", "catcarrier"}

// InitShippingClient wires the HTTP client used by CheckoutHandler to call
// the shipping service. peer.service=shipping so the service graph shows
// the cart→shipping edge under a stable name.
func InitShippingClient(baseURL string) {
	shippingBaseURL = baseURL
	shippingClient = &http.Client{
		Timeout: 5 * time.Second,
		Transport: otelhttp.NewTransport(
			http.DefaultTransport,
			otelhttp.WithSpanOptions(trace.WithAttributes(attribute.String("peer.service", "shipping"))),
		),
	}
}

// GetShippingRates calls GET /api/shipping/rates. Best-effort: the trace
// includes the call so the service graph shows the edge, but a rates
// failure does not block checkout.
func GetShippingRates(ctx context.Context) error {
	if shippingClient == nil {
		return fmt.Errorf("shipping client not initialized")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		shippingBaseURL+"/api/shipping/rates", nil)
	if err != nil {
		return fmt.Errorf("build rates request: %w", err)
	}
	resp, err := shippingClient.Do(req)
	if err != nil {
		return fmt.Errorf("shipping rates: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.WarnContext(ctx, "Failed to close rates response body", "error", err)
		}
	}()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		slog.WarnContext(ctx, "Failed to drain rates response body", "error", err)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("shipping rates returned %d", resp.StatusCode)
	}
	return nil
}

type shipRequest struct {
	OrderID string `json:"order_id"`
	Carrier string `json:"carrier,omitempty"`
}

type shipResponse struct {
	ShipmentID  string  `json:"shipment_id"`
	Status      string  `json:"status"`
	Carrier     string  `json:"carrier"`
	CarrierName string  `json:"carrier_name"`
	Cost        float64 `json:"cost"`
	Error       string  `json:"error_message,omitempty"`
}

// CreateShipment posts to POST /api/shipping/ship. A non-2xx status code
// (the shipping service returns 503 when the carrier declines) is returned
// as an error so the caller can fail the checkout.
func CreateShipment(ctx context.Context, orderID string) (*shipResponse, error) {
	if shippingClient == nil {
		return nil, fmt.Errorf("shipping client not initialized")
	}

	carrier := shippingCarriers[rand.Intn(len(shippingCarriers))]
	body, err := json.Marshal(shipRequest{OrderID: orderID, Carrier: carrier})
	if err != nil {
		return nil, fmt.Errorf("marshal ship request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		shippingBaseURL+"/api/shipping/ship", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build ship request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := shippingClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("shipping ship: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.WarnContext(ctx, "Failed to close ship response body", "error", err)
		}
	}()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read ship response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("shipping %s returned %d: %s", carrier, resp.StatusCode, raw)
	}

	var parsed shipResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse ship response: %w", err)
	}
	return &parsed, nil
}
