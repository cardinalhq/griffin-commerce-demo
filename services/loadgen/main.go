// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

// Package loadgen drives a continuous low-rate checkout flow against the
// Griffin cart service for the Airtel demo. Each iteration creates a cart,
// adds 1-2 items, and checks out — producing a multi-service trace that
// includes a payment.charge span. When dbaas.disk-full is active in the
// controlplane, payment's db.write child span returns SQLSTATE 53100,
// which is exactly what the customer-persona investigate flow surfaces.
package loadgen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/cardinalhq/griffin-commerce-demo/common"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// products is the catalog the loadgen randomly picks from. Matches the
// product IDs in services/catalog/products.yaml so cart.add doesn't 404.
var products = []string{"PROD-001", "PROD-002", "PROD-003", "PROD-004", "PROD-005"}

func Start() error {
	ctx, shutdown, err := common.SetupTelemetry("loadgen-service", nil)
	if err != nil {
		return fmt.Errorf("init telemetry: %w", err)
	}
	defer func() {
		if err := shutdown(); err != nil {
			slog.Error("Failed to shutdown telemetry", "error", err)
		}
	}()

	httpClient := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
		Timeout:   10 * time.Second,
	}

	cartURL := envOr("CART_URL", "http://cart:8082")
	paymentURL := envOr("PAYMENT_URL", "http://payment:8081")
	rps := envFloatOr("LOADGEN_RPS", 1.0)
	interval := time.Duration(float64(time.Second) / rps)
	if interval < 100*time.Millisecond {
		interval = 100 * time.Millisecond
	}

	slog.InfoContext(ctx, "loadgen started",
		"cart_url", cartURL,
		"payment_url", paymentURL,
		"rps", rps,
		"interval_ms", interval.Milliseconds(),
	)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "loadgen shutting down")
			return nil
		case <-ticker.C:
			// Fire-and-forget; each session is independent.
			go runCheckoutSession(ctx, httpClient, cartURL, paymentURL)
		}
	}
}

// runCheckoutSession exercises the full SmartHub flow:
//   create cart → add items → cart checkout → payment charge.
// The cart and payment services are independent in this demo, so we have
// to invoke both legs explicitly — but both calls share the same trace
// because the otelhttp transport propagates traceparent. Result: each
// session yields one multi-service trace whose payment leg fails with
// SQLSTATE 53100 when dbaas.disk-full is active.
func runCheckoutSession(ctx context.Context, c *http.Client, cartBase, paymentBase string) {
	cartID, err := createCart(ctx, c, cartBase)
	if err != nil {
		slog.WarnContext(ctx, "create cart failed", "error", err)
		return
	}
	total := 0.0
	itemCount := rand.Intn(2) + 1
	for i := 0; i < itemCount; i++ {
		prod := products[rand.Intn(len(products))]
		qty := rand.Intn(2) + 1
		if err := addItem(ctx, c, cartBase, cartID, prod, qty); err != nil {
			slog.WarnContext(ctx, "add item failed", "error", err, "product", prod)
			return
		}
		// Approximate per-item price for the synthetic charge amount.
		total += float64(qty) * (10.0 + rand.Float64()*50.0)
	}
	if err := checkout(ctx, c, cartBase, cartID); err != nil {
		slog.DebugContext(ctx, "checkout failed", "error", err, "cart_id", cartID)
		return
	}
	if err := chargePayment(ctx, c, paymentBase, cartID, total); err != nil {
		// Expected when dbaas.disk-full is active — log at debug to keep
		// steady-state logs clean.
		slog.DebugContext(ctx, "charge failed", "error", err, "cart_id", cartID)
	}
}

// chargePayment calls POST /api/payments/charge — the call that exercises
// RecordOrder() in services/payment/db.go, which is what returns SQLSTATE
// 53100 when the dbaas.disk-full knob is active.
func chargePayment(ctx context.Context, c *http.Client, base, orderID string, amount float64) error {
	body, _ := json.Marshal(map[string]any{
		"order_id":  orderID,
		"amount":    amount,
		"processor": "PuppyPay",
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		base+"/api/payments/charge", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("charge returned %d", resp.StatusCode)
	}
	return nil
}

// createCart calls POST /api/cart/create. Cart requires a customer_id —
// we generate a synthetic SmartHub end-user id per session so each trace
// looks like a distinct shopper.
func createCart(ctx context.Context, c *http.Client, base string) (string, error) {
	endUserID := fmt.Sprintf("smarthub-user-%04d", rand.Intn(10000))
	reqBody, _ := json.Marshal(map[string]string{"customer_id": endUserID})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/cart/create", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("create cart returned %d: %s", resp.StatusCode, body)
	}
	var parsed struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("create cart: parse response: %w", err)
	}
	if parsed.ID == "" {
		return "", fmt.Errorf("create cart: empty id in response: %s", body)
	}
	return parsed.ID, nil
}

func addItem(ctx context.Context, c *http.Client, base, cartID, productID string, qty int) error {
	body, _ := json.Marshal(map[string]any{"product_id": productID, "quantity": qty})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/api/cart/%s/add", base, cartID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("add item returned %d", resp.StatusCode)
	}
	return nil
}

func checkout(ctx context.Context, c *http.Client, base, cartID string) error {
	body, _ := json.Marshal(map[string]any{
		"payment_method": "credit_card",
		"shipping_address": map[string]string{
			"street": "123 Test St",
			"city":   "Mumbai",
			"state":  "MH",
			"zip":    "400001",
		},
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/api/cart/%s/checkout", base, cartID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("checkout returned %d", resp.StatusCode)
	}
	return nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envFloatOr(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}
