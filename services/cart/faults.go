// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package cart

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/cardinalhq/griffin-commerce-demo/common"
	"github.com/cardinalhq/griffin-commerce-demo/common/faults"
)

// maybeFailCartError checks the cart.error knob and, if it fires, writes
// the configured error response and returns true. Caller must return.
// statusCode out-param is updated for the deferred metric emission.
func maybeFailCartError(w http.ResponseWriter, r *http.Request, operation string, statusCode *int) bool {
	k, fired := faults.Probe(faultsClient, "cart.error")
	if !fired {
		return false
	}
	code := k.StatusCode
	if code == 0 {
		code = http.StatusInternalServerError
	}
	*statusCode = code
	slog.ErrorContext(r.Context(), "cart: returning fault-injected error",
		"griffin.fault", k.Key,
		"operation", operation,
		"status_code", code,
	)
	faults.Record(r.Context(), k, 0)
	correlationID := common.GetCorrelationID(r.Context())
	common.WriteErrorResponse(r.Context(), w,
		common.NewAppError("CART_UNAVAILABLE", "Cart service temporarily unavailable"),
		code, correlationID)
	return true
}

// maybeFailPoisonProduct checks whether the cart at cartID contains the
// poison-product target and, if so, returns a deliberately uninformative
// 500 while logging the actual cause with trace_id. This is the canonical
// "logs explain the failure" demo — the trace shows a 500, the logs side
// drawer (filtered by trace_id) shows the structured "cart contains
// tainted item" message naming the product.
func maybeFailPoisonProduct(w http.ResponseWriter, r *http.Request, cartID, operation string, statusCode *int) bool {
	k, fired := faults.Probe(faultsClient, "cart.poison-product")
	if !fired {
		return false
	}
	target := k.Target
	if target == "" {
		return false
	}

	cart, err := GetCart(cartID)
	if err != nil {
		// Cart doesn't exist; nothing to poison.
		return false
	}
	hasPoison := false
	for _, item := range cart.Items {
		if item.ProductID == target {
			hasPoison = true
			break
		}
	}
	if !hasPoison {
		return false
	}

	*statusCode = http.StatusInternalServerError
	slog.ErrorContext(r.Context(), "cart contains tainted item: poison product detected",
		"griffin.fault", k.Key,
		"operation", operation,
		"cart_id", cartID,
		"product_id", target,
		"cause", "data integrity check failed: corrupted item record in cart",
	)
	faults.Record(r.Context(), k, 0)

	// Deliberately bland response — the cause is only in the logs.
	correlationID := common.GetCorrelationID(r.Context())
	common.WriteErrorResponse(r.Context(), w,
		common.NewAppError("INTERNAL_ERROR", "The service encountered an unexpected error"),
		http.StatusInternalServerError, correlationID)
	return true
}

// maybeSkipReversal checks the cart.skip-reversal knob. When it fires, the
// compensating ReversePayment call is skipped and the checkout leaves an
// authorized charge stranded — the customer is billed for an order that
// never ships.
//
// This is the demo's behavioral-contract regression. Every span in the
// resulting trace still reports accurately: payment 200, shipping 500,
// checkout 502. The passing and violating executions are span-for-span
// identical; only the absence of the reversal event separates them, which
// is precisely what a behavioral contract detects and a span query cannot.
func maybeSkipReversal(ctx context.Context, orderID, transactionID string) bool {
	k, fired := faults.Probe(faultsClient, "cart.skip-reversal")
	if !fired {
		return false
	}
	slog.ErrorContext(ctx, "checkout skipped payment reversal: charge left authorized",
		"griffin.fault", k.Key,
		"cart_id", orderID,
		"payment_transaction_id", transactionID,
	)
	faults.Record(ctx, k, 0)
	return true
}
