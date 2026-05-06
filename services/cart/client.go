// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package cart

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/cardinalhq/griffin-commerce-demo/common"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var (
	httpClient     *http.Client
	catalogBaseURL string
)

// InitProductClient initializes the HTTP client for the product service
func InitProductClient(baseURL string) {
	catalogBaseURL = baseURL
	httpClient = &http.Client{
		Timeout: 5 * time.Second,
		Transport: otelhttp.NewTransport(
			http.DefaultTransport,
			otelhttp.WithSpanOptions(trace.WithAttributes(attribute.String("peer.service", "catalog"))),
		),
	}
}

// GetProduct fetches product details from the catalog service
func GetProduct(ctx context.Context, productID string) (*common.Product, error) {
	url := fmt.Sprintf("%s/api/products/%s", catalogBaseURL, productID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch product: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.WarnContext(ctx, "Failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("product not found: %s", productID)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch product, status: %d", resp.StatusCode)
	}

	var product common.Product
	if err := json.NewDecoder(resp.Body).Decode(&product); err != nil {
		return nil, fmt.Errorf("failed to decode product response: %w", err)
	}

	return &product, nil
}
