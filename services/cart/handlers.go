// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package cart

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/cardinalhq/griffin-commerce-demo/common"
	"github.com/cardinalhq/griffin-commerce-demo/common/faults"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// RegisterRoutes registers all HTTP routes
func RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/health", HealthHandler).Methods("GET")
	r.HandleFunc("/api/cart/create", CreateCartHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/cart/{id}", GetCartHandler).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/cart/{id}/add", AddItemHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/cart/{id}/item/{productId}", RemoveItemHandler).Methods("DELETE", "OPTIONS")
	r.HandleFunc("/api/cart/{id}/clear", ClearCartHandler).Methods("DELETE", "OPTIONS")
	r.HandleFunc("/api/cart/{id}/checkout", CheckoutHandler).Methods("POST", "OPTIONS")
}

// HealthHandler returns service health status
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	health := common.HealthResponse{
		Status:    "healthy",
		Service:   "cart-service",
		Version:   "1.0.0",
		Timestamp: time.Now(),
	}
	if err := common.WriteJSONResponse(w, health, http.StatusOK); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write health response", "error", err)
	}
}

// CreateCartRequest represents a cart creation request
type CreateCartRequest struct {
	CustomerID string `json:"customer_id"`
}

// CreateCartHandler creates a new cart.
func CreateCartHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	statusCode := http.StatusCreated
	defer func() {
		faults.RecordCartOp(r.Context(), "create", statusCode, float64(time.Since(start).Milliseconds()))
	}()

	if maybeFailCartError(w, r, "create", &statusCode) {
		return
	}
	faults.MaybeOutlier(r.Context(), faultsClient, "cart.outlier")

	var req CreateCartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		statusCode = http.StatusBadRequest
		correlationID := common.GetCorrelationID(r.Context())
		common.WriteErrorResponse(r.Context(), w, common.ErrBadRequest, statusCode, correlationID)
		return
	}

	if req.CustomerID == "" {
		statusCode = http.StatusBadRequest
		correlationID := common.GetCorrelationID(r.Context())
		err := common.NewAppError("MISSING_CUSTOMER_ID", "Customer ID is required")
		common.WriteErrorResponse(r.Context(), w, err, statusCode, correlationID)
		return
	}

	cart, err := CreateCart(req.CustomerID)
	if err != nil {
		statusCode = http.StatusInternalServerError
		correlationID := common.GetCorrelationID(r.Context())
		appErr := common.NewAppError("CART_CREATION_FAILED", err.Error())
		common.WriteErrorResponse(r.Context(), w, appErr, statusCode, correlationID)
		return
	}

	trace.SpanFromContext(r.Context()).SetAttributes(
		attribute.String("cart.id", cart.ID),
		attribute.String("cart.customer_id", cart.CustomerID),
	)
	slog.InfoContext(r.Context(), "cart created",
		"cart_id", cart.ID, "customer_id", cart.CustomerID,
	)

	if err := common.WriteJSONResponse(w, cart, http.StatusCreated); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write cart response", "error", err)
	}
}

// GetCartHandler retrieves a cart.
func GetCartHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	statusCode := http.StatusOK
	vars := mux.Vars(r)
	cartID := vars["id"]
	span := trace.SpanFromContext(r.Context())
	span.SetAttributes(attribute.String("cart.id", cartID))
	defer func() {
		faults.RecordCartOp(r.Context(), "get", statusCode, float64(time.Since(start).Milliseconds()))
	}()

	if maybeFailCartError(w, r, "get", &statusCode) {
		return
	}
	if maybeFailPoisonProduct(w, r, cartID, "get", &statusCode) {
		return
	}
	faults.MaybeOutlier(r.Context(), faultsClient, "cart.outlier")

	cart, err := GetCart(cartID)
	if err != nil {
		statusCode = http.StatusNotFound
		correlationID := common.GetCorrelationID(r.Context())
		common.WriteErrorResponse(r.Context(), w, common.ErrNotFound, statusCode, correlationID)
		return
	}

	span.SetAttributes(
		attribute.Int("cart.item_count", len(cart.Items)),
		attribute.Float64("cart.total", cart.Total),
	)
	slog.InfoContext(r.Context(), "cart viewed",
		"cart_id", cart.ID, "item_count", len(cart.Items), "total", cart.Total,
	)

	if err := common.WriteJSONResponse(w, cart, http.StatusOK); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write cart response", "error", err)
	}
}

// AddItemRequest represents an add item request
type AddItemRequest struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

// AddItemHandler adds an item to the cart.
// Fault hooks: cart.error, cart.poison-product (checked against cart's
// pre-add state — adding the poison item itself succeeds the first time;
// subsequent operations on the now-poisoned cart fail).
func AddItemHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	statusCode := http.StatusOK
	vars := mux.Vars(r)
	cartID := vars["id"]
	span := trace.SpanFromContext(r.Context())
	span.SetAttributes(attribute.String("cart.id", cartID))
	defer func() {
		faults.RecordCartOp(r.Context(), "add", statusCode, float64(time.Since(start).Milliseconds()))
	}()

	if maybeFailCartError(w, r, "add", &statusCode) {
		return
	}
	if maybeFailPoisonProduct(w, r, cartID, "add", &statusCode) {
		return
	}
	faults.MaybeOutlier(r.Context(), faultsClient, "cart.outlier")

	var req AddItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		statusCode = http.StatusBadRequest
		correlationID := common.GetCorrelationID(r.Context())
		common.WriteErrorResponse(r.Context(), w, common.ErrBadRequest, statusCode, correlationID)
		return
	}

	if req.ProductID == "" || req.Quantity <= 0 {
		statusCode = http.StatusBadRequest
		correlationID := common.GetCorrelationID(r.Context())
		err := common.NewAppError("INVALID_REQUEST", "Product ID and positive quantity are required")
		common.WriteErrorResponse(r.Context(), w, err, statusCode, correlationID)
		return
	}

	span.SetAttributes(
		attribute.String("product.id", req.ProductID),
		attribute.Int("product.quantity", req.Quantity),
	)

	if err := AddItemToCart(r.Context(), cartID, req.ProductID, req.Quantity); err != nil {
		correlationID := common.GetCorrelationID(r.Context())
		statusCode = http.StatusInternalServerError
		appErr := common.NewAppError("ADD_ITEM_FAILED", err.Error())

		if err.Error() == "product not found" {
			statusCode = http.StatusNotFound
			appErr = common.ErrNotFound
		}

		common.WriteErrorResponse(r.Context(), w, appErr, statusCode, correlationID)
		return
	}

	// Return updated cart
	cart, _ := GetCart(cartID)
	span.SetAttributes(
		attribute.Int("cart.item_count", len(cart.Items)),
		attribute.Float64("cart.total", cart.Total),
	)
	slog.InfoContext(r.Context(), "cart item added",
		"cart_id", cart.ID,
		"product_id", req.ProductID,
		"quantity", req.Quantity,
		"item_count", len(cart.Items),
		"total", cart.Total,
	)
	if err := common.WriteJSONResponse(w, cart, http.StatusOK); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write cart response", "error", err)
	}
}

// RemoveItemHandler removes an item from the cart.
// We deliberately do NOT apply cart.poison-product here so operators can
// remove the poison item to recover.
func RemoveItemHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	statusCode := http.StatusOK
	vars := mux.Vars(r)
	cartID := vars["id"]
	productID := vars["productId"]
	span := trace.SpanFromContext(r.Context())
	span.SetAttributes(
		attribute.String("cart.id", cartID),
		attribute.String("product.id", productID),
	)
	defer func() {
		faults.RecordCartOp(r.Context(), "remove", statusCode, float64(time.Since(start).Milliseconds()))
	}()

	if maybeFailCartError(w, r, "remove", &statusCode) {
		return
	}
	faults.MaybeOutlier(r.Context(), faultsClient, "cart.outlier")

	if err := RemoveItemFromCart(cartID, productID); err != nil {
		correlationID := common.GetCorrelationID(r.Context())
		statusCode = http.StatusInternalServerError
		appErr := common.NewAppError("REMOVE_ITEM_FAILED", err.Error())

		if err.Error() == "cart not found" || err.Error() == "item not found in cart" {
			statusCode = http.StatusNotFound
			appErr = common.ErrNotFound
		}

		common.WriteErrorResponse(r.Context(), w, appErr, statusCode, correlationID)
		return
	}

	cart, _ := GetCart(cartID)
	span.SetAttributes(
		attribute.Int("cart.item_count", len(cart.Items)),
		attribute.Float64("cart.total", cart.Total),
	)
	slog.InfoContext(r.Context(), "cart item removed",
		"cart_id", cart.ID, "product_id", productID,
		"item_count", len(cart.Items), "total", cart.Total,
	)
	if err := common.WriteJSONResponse(w, cart, http.StatusOK); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write cart response", "error", err)
	}
}

// CheckoutResponse represents a checkout response
type CheckoutResponse struct {
	CartID     string  `json:"cart_id"`
	CustomerID string  `json:"customer_id"`
	Total      float64 `json:"total"`
	ItemCount  int     `json:"item_count"`
	Message    string  `json:"message"`
}

// CheckoutHandler initiates checkout for a cart and drives the downstream
// fan-out: shipping.rates → payment.charge → shipping.ship. Payment and
// ship failures propagate as 502 so the service graph shows the real
// dependency and the failed-cart trace roots at cart with error children.
func CheckoutHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	statusCode := http.StatusOK
	vars := mux.Vars(r)
	cartID := vars["id"]
	span := trace.SpanFromContext(r.Context())
	span.SetAttributes(attribute.String("cart.id", cartID))
	defer func() {
		faults.RecordCartOp(r.Context(), "checkout", statusCode, float64(time.Since(start).Milliseconds()))
	}()

	if maybeFailCartError(w, r, "checkout", &statusCode) {
		return
	}
	faults.MaybeOutlier(r.Context(), faultsClient, "cart.outlier")

	cart, err := GetCart(cartID)
	if err != nil {
		statusCode = http.StatusNotFound
		correlationID := common.GetCorrelationID(r.Context())
		common.WriteErrorResponse(r.Context(), w, common.ErrNotFound, statusCode, correlationID)
		return
	}

	if len(cart.Items) == 0 {
		statusCode = http.StatusBadRequest
		correlationID := common.GetCorrelationID(r.Context())
		err := common.NewAppError("EMPTY_CART", "Cannot checkout an empty cart")
		common.WriteErrorResponse(r.Context(), w, err, statusCode, correlationID)
		return
	}

	span.SetAttributes(
		attribute.String("cart.customer_id", cart.CustomerID),
		attribute.Int("cart.item_count", len(cart.Items)),
		attribute.Float64("cart.total", cart.Total),
	)

	// Shipping rates: best-effort, informational span. A rates failure
	// shouldn't block checkout — real e-commerce falls back to a default
	// rate — but we log so it's visible in the trace.
	if err := GetShippingRates(r.Context()); err != nil {
		slog.WarnContext(r.Context(), "shipping rates unavailable, continuing",
			"cart_id", cart.ID, "error", err,
		)
	}

	// Payment charge. Non-2xx (including 402 from the processor's baseline
	// failure_rate) fails the whole checkout with 502 — the customer sees
	// a failed checkout, the trace shows the failing payment child span,
	// and the payment.processor attribute reveals which processor rejected.
	charge, err := ChargePayment(r.Context(), cart.ID, cart.Total)
	if err != nil {
		statusCode = http.StatusBadGateway
		correlationID := common.GetCorrelationID(r.Context())
		appErr := common.NewAppError("PAYMENT_FAILED", err.Error())
		slog.ErrorContext(r.Context(), "checkout payment failed",
			"cart_id", cart.ID, "customer_id", cart.CustomerID, "error", err,
		)
		common.WriteErrorResponse(r.Context(), w, appErr, statusCode, correlationID)
		return
	}
	span.SetAttributes(
		attribute.String("payment.transaction_id", charge.TransactionID),
		attribute.String("payment.processor", charge.Processor),
	)

	// Shipment creation. Same treatment as payment: non-2xx (e.g. carrier
	// declined per baseline failure_rate) fails the checkout with 502.
	//
	// The card is already authorized at this point, so a shipping failure
	// obliges us to reverse the charge. A checkout that returns 502 without
	// either confirming the order or reversing the payment has billed the
	// customer for an order that will never ship.
	ship, err := CreateShipment(r.Context(), cart.ID)
	if err != nil {
		statusCode = http.StatusBadGateway
		compensateFailedCheckout(r.Context(), span, cart.ID, charge.TransactionID)

		correlationID := common.GetCorrelationID(r.Context())
		appErr := common.NewAppError("SHIPPING_FAILED", err.Error())
		slog.ErrorContext(r.Context(), "checkout shipping failed",
			"cart_id", cart.ID, "customer_id", cart.CustomerID,
			"payment_transaction_id", charge.TransactionID, "error", err,
		)
		common.WriteErrorResponse(r.Context(), w, appErr, statusCode, correlationID)
		return
	}
	// The order is confirmed: payment authorized and shipment created. This
	// is the other legal terminal state of the checkout protocol, and the
	// counterpart to the "reversed" branch above.
	span.SetAttributes(
		attribute.String("shipping.shipment_id", ship.ShipmentID),
		attribute.String("shipping.carrier", ship.Carrier),
		attribute.String("checkout.terminal_state", "confirmed"),
		attribute.String("checkout.order_id", cart.ID),
	)

	response := CheckoutResponse{
		CartID:     cart.ID,
		CustomerID: cart.CustomerID,
		Total:      cart.Total,
		ItemCount:  len(cart.Items),
		Message:    "Checkout complete",
	}

	slog.InfoContext(r.Context(), "checkout complete",
		"cart_id", cart.ID, "customer_id", cart.CustomerID,
		"item_count", len(cart.Items), "total", cart.Total,
		"payment_transaction_id", charge.TransactionID,
		"payment_processor", charge.Processor,
		"shipment_id", ship.ShipmentID,
		"shipping_carrier", ship.Carrier,
	)

	if err := common.WriteJSONResponse(w, response, http.StatusOK); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write checkout response", "error", err)
	}
}

// compensateFailedCheckout releases the authorized charge for a checkout
// that failed after payment succeeded. It records the outcome on the
// checkout span so the trace states which terminal branch the execution
// reached: checkout.terminal_state ∈ {reversed, reversal_failed, none}.
//
// "none" is a protocol violation, not a service error — the customer's
// money is held for an order that will never ship, and no individual span
// in the trace reports a problem beyond the shipping 500 that caused it.
func compensateFailedCheckout(ctx context.Context, span trace.Span, orderID, transactionID string) {
	if maybeSkipReversal(ctx, orderID, transactionID) {
		span.SetAttributes(attribute.String("checkout.terminal_state", "none"))
		return
	}

	if _, err := ReversePayment(ctx, orderID, transactionID); err != nil {
		span.SetAttributes(
			attribute.String("checkout.terminal_state", "reversal_failed"),
			attribute.String("checkout.reversal_error", err.Error()),
		)
		slog.ErrorContext(ctx, "checkout could not reverse payment: charge left authorized",
			"cart_id", orderID, "payment_transaction_id", transactionID, "error", err,
		)
		return
	}

	span.SetAttributes(attribute.String("checkout.terminal_state", "reversed"))
	slog.InfoContext(ctx, "checkout reversed payment after downstream failure",
		"cart_id", orderID, "payment_transaction_id", transactionID,
	)
}

// ClearCartHandler clears all items from a cart.
func ClearCartHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	statusCode := http.StatusOK
	vars := mux.Vars(r)
	cartID := vars["id"]
	span := trace.SpanFromContext(r.Context())
	span.SetAttributes(attribute.String("cart.id", cartID))
	defer func() {
		faults.RecordCartOp(r.Context(), "clear", statusCode, float64(time.Since(start).Milliseconds()))
	}()

	if maybeFailCartError(w, r, "clear", &statusCode) {
		return
	}
	faults.MaybeOutlier(r.Context(), faultsClient, "cart.outlier")

	cart, err := GetCart(cartID)
	if err != nil {
		statusCode = http.StatusNotFound
		correlationID := common.GetCorrelationID(r.Context())
		common.WriteErrorResponse(r.Context(), w, common.ErrNotFound, statusCode, correlationID)
		return
	}

	cart.Items = []common.CartItem{}
	cart.Total = 0
	cart.UpdatedAt = time.Now()

	if err := cartDB.Set(cart.ID, cart); err != nil {
		statusCode = http.StatusInternalServerError
		correlationID := common.GetCorrelationID(r.Context())
		appErr := common.NewAppError("CLEAR_FAILED", "Failed to clear cart")
		common.WriteErrorResponse(r.Context(), w, appErr, statusCode, correlationID)
		return
	}

	slog.InfoContext(r.Context(), "cart cleared", "cart_id", cart.ID)

	if err := common.WriteJSONResponse(w, cart, http.StatusOK); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write cart response", "error", err)
	}
}
