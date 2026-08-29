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
	paymentClient  *http.Client
	paymentBaseURL string
)

// paymentProcessors matches the keys in the top-level config.yaml baked into
// the image. Rotating through all three so per-processor failure rates
// (5% / 2% / 1% baseline; the payment.fail knob targets a single one) are
// exercised — previously loadgen hardcoded PuppyPay and the other two saw no
// traffic.
var paymentProcessors = []string{"PuppyPay", "KittyCard", "DoggieCoin"}

// InitPaymentClient wires the HTTP client used by CheckoutHandler to call
// the payment service. peer.service=payment so the service graph shows the
// cart→payment edge under a stable name regardless of DNS.
func InitPaymentClient(baseURL string) {
	paymentBaseURL = baseURL
	paymentClient = &http.Client{
		Timeout: 5 * time.Second,
		Transport: otelhttp.NewTransport(
			http.DefaultTransport,
			otelhttp.WithSpanOptions(trace.WithAttributes(attribute.String("peer.service", "payment"))),
		),
	}
}

type chargeRequest struct {
	OrderID   string  `json:"order_id"`
	Amount    float64 `json:"amount"`
	Processor string  `json:"processor,omitempty"`
}

type chargeResponse struct {
	TransactionID string  `json:"transaction_id"`
	Status        string  `json:"status"`
	Processor     string  `json:"processor"`
	Message       string  `json:"message"`
	Amount        float64 `json:"amount"`
}

// ChargePayment posts to POST /api/payments/charge and returns the parsed
// response. A non-2xx status code (including 402 "payment failed") is
// returned as an error so the caller can surface it up to the client.
func ChargePayment(ctx context.Context, orderID string, amount float64) (*chargeResponse, error) {
	if paymentClient == nil {
		return nil, fmt.Errorf("payment client not initialized")
	}

	processor := paymentProcessors[rand.Intn(len(paymentProcessors))]
	body, err := json.Marshal(chargeRequest{
		OrderID:   orderID,
		Amount:    amount,
		Processor: processor,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal charge request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		paymentBaseURL+"/api/payments/charge", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build charge request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := paymentClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("payment charge: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.WarnContext(ctx, "Failed to close payment response body", "error", err)
		}
	}()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read charge response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("payment %s returned %d: %s", processor, resp.StatusCode, raw)
	}

	var parsed chargeResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse charge response: %w", err)
	}
	return &parsed, nil
}

type reverseRequest struct {
	OrderID       string `json:"order_id"`
	TransactionID string `json:"transaction_id"`
}

// ReversePayment posts to POST /api/payments/reverse to release a charge
// that was authorized for a checkout which then failed downstream. This is
// the compensating leg of the checkout protocol: without it the customer is
// charged for an order that never ships.
func ReversePayment(ctx context.Context, orderID, transactionID string) (*chargeResponse, error) {
	if paymentClient == nil {
		return nil, fmt.Errorf("payment client not initialized")
	}

	body, err := json.Marshal(reverseRequest{
		OrderID:       orderID,
		TransactionID: transactionID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal reverse request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		paymentBaseURL+"/api/payments/reverse", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build reverse request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := paymentClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("payment reverse: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.WarnContext(ctx, "Failed to close reversal response body", "error", err)
		}
	}()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read reverse response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("payment reverse returned %d: %s", resp.StatusCode, raw)
	}

	var parsed chargeResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse reverse response: %w", err)
	}
	return &parsed, nil
}
