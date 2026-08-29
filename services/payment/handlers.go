// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package payment

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/cardinalhq/griffin-commerce-demo/common"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// RegisterRoutes registers all HTTP routes
func RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/health", HealthHandler).Methods("GET")
	r.HandleFunc("/api/payments/charge", ChargeHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/payments/reverse", ReverseHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/payments/{id}", GetTransactionHandler).Methods("GET", "OPTIONS")
}

// HealthHandler returns service health status
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	health := common.HealthResponse{
		Status:    "healthy",
		Service:   "payment-service",
		Version:   "1.0.0",
		Timestamp: time.Now(),
	}
	if err := common.WriteJSONResponse(w, health, http.StatusOK); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write health response", "error", err)
	}
}

// ChargeRequest represents a payment charge request
type ChargeRequest struct {
	OrderID   string  `json:"order_id"`
	Amount    float64 `json:"amount"`
	Processor string  `json:"processor,omitempty"` // Optional, random if not specified
}

// ChargeResponse represents a payment charge response
type ChargeResponse struct {
	TransactionID string    `json:"transaction_id"`
	OrderID       string    `json:"order_id"`
	Amount        float64   `json:"amount"`
	Status        string    `json:"status"`
	Processor     string    `json:"processor"`
	Message       string    `json:"message"`
	CreatedAt     time.Time `json:"created_at"`
}

// ChargeHandler processes a payment charge
func ChargeHandler(w http.ResponseWriter, r *http.Request) {
	var req ChargeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		correlationID := common.GetCorrelationID(r.Context())
		common.WriteErrorResponse(r.Context(), w, common.ErrBadRequest, http.StatusBadRequest, correlationID)
		return
	}

	// Validate request
	if req.OrderID == "" || req.Amount <= 0 {
		correlationID := common.GetCorrelationID(r.Context())
		err := common.NewAppError("INVALID_REQUEST", "Order ID and positive amount are required")
		common.WriteErrorResponse(r.Context(), w, err, http.StatusBadRequest, correlationID)
		return
	}

	span := trace.SpanFromContext(r.Context())
	span.SetAttributes(
		attribute.String("payment.order_id", req.OrderID),
		attribute.Float64("payment.amount", req.Amount),
	)
	if req.Processor != "" {
		span.SetAttributes(attribute.String("payment.requested_processor", req.Processor))
	}
	slog.InfoContext(r.Context(), "payment charge initiated",
		"order_id", req.OrderID, "amount", req.Amount, "requested_processor", req.Processor,
	)

	// Record the order on the simulated DBaaS instance before processing
	// the charge. When the dbaas.disk-full knob is active, this returns
	// SQLSTATE 53100 and short-circuits the charge — drives the customer-
	// persona half of the Airtel demo.
	if err := RecordOrder(r.Context(), req.OrderID, req.Amount); err != nil {
		correlationID := common.GetCorrelationID(r.Context())
		appErr := common.NewAppError("DB_ERROR", err.Error())
		common.WriteErrorResponse(r.Context(), w, appErr, http.StatusServiceUnavailable, correlationID)
		return
	}

	// Process payment
	transaction, err := ProcessPayment(r.Context(), req.OrderID, req.Amount, req.Processor)
	if err != nil {
		correlationID := common.GetCorrelationID(r.Context())
		appErr := common.NewAppError("PROCESSING_ERROR", err.Error())
		common.WriteErrorResponse(r.Context(), w, appErr, http.StatusInternalServerError, correlationID)
		return
	}

	// Create response
	response := ChargeResponse{
		TransactionID: transaction.ID,
		OrderID:       transaction.OrderID,
		Amount:        transaction.Amount,
		Status:        transaction.Status,
		Processor:     transaction.Processor,
		Message:       transaction.Message,
		CreatedAt:     transaction.CreatedAt,
	}

	// Return appropriate status code based on payment result
	statusCode := http.StatusOK
	if transaction.Status == "failed" {
		statusCode = http.StatusPaymentRequired // 402
	}

	span.SetAttributes(
		attribute.String("payment.transaction_id", transaction.ID),
		attribute.String("payment.processor", transaction.Processor),
		attribute.String("payment.status", transaction.Status),
	)

	if err := common.WriteJSONResponse(w, response, statusCode); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write charge response", "error", err, "status", statusCode)
	}
}

// ReverseRequest represents a request to reverse a previously successful charge.
type ReverseRequest struct {
	OrderID       string `json:"order_id"`
	TransactionID string `json:"transaction_id"`
}

// ReverseHandler reverses a charge. Called by cart when a checkout fails
// after payment succeeded; see services/cart/handlers.go CheckoutHandler.
//
// The span attributes here are the observable facts that make the
// "authorized payment must terminate" behavioral contract decidable:
// payment.order_id correlates to the charge, and payment.status=reversed
// is the terminal state. Do not remove them without updating that contract.
func ReverseHandler(w http.ResponseWriter, r *http.Request) {
	var req ReverseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		correlationID := common.GetCorrelationID(r.Context())
		common.WriteErrorResponse(r.Context(), w, common.ErrBadRequest, http.StatusBadRequest, correlationID)
		return
	}

	if req.OrderID == "" || req.TransactionID == "" {
		correlationID := common.GetCorrelationID(r.Context())
		err := common.NewAppError("INVALID_REQUEST", "Order ID and transaction ID are required")
		common.WriteErrorResponse(r.Context(), w, err, http.StatusBadRequest, correlationID)
		return
	}

	span := trace.SpanFromContext(r.Context())
	span.SetAttributes(
		attribute.String("payment.order_id", req.OrderID),
		attribute.String("payment.transaction_id", req.TransactionID),
	)
	slog.InfoContext(r.Context(), "payment reversal initiated",
		"order_id", req.OrderID, "transaction_id", req.TransactionID,
	)

	transaction, outcome, err := ReversePayment(r.Context(), req.OrderID, req.TransactionID)
	if err != nil {
		span.SetAttributes(attribute.String("payment.reversal_outcome", outcome))
		slog.ErrorContext(r.Context(), "payment reversal failed",
			"order_id", req.OrderID, "transaction_id", req.TransactionID, "error", err,
		)
		correlationID := common.GetCorrelationID(r.Context())
		appErr := common.NewAppError("REVERSAL_FAILED", err.Error())
		common.WriteErrorResponse(r.Context(), w, appErr, http.StatusUnprocessableEntity, correlationID)
		return
	}

	span.SetAttributes(
		attribute.Float64("payment.amount", transaction.Amount),
		attribute.String("payment.processor", transaction.Processor),
		attribute.String("payment.status", transaction.Status),
		attribute.String("payment.reversal_outcome", outcome),
	)

	response := ChargeResponse{
		TransactionID: transaction.ID,
		OrderID:       transaction.OrderID,
		Amount:        transaction.Amount,
		Status:        transaction.Status,
		Processor:     transaction.Processor,
		Message:       transaction.Message,
		CreatedAt:     transaction.CreatedAt,
	}

	if err := common.WriteJSONResponse(w, response, http.StatusOK); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write reversal response", "error", err)
	}
}

// GetTransactionHandler retrieves a transaction by ID
func GetTransactionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	transactionID := vars["id"]

	trace.SpanFromContext(r.Context()).SetAttributes(
		attribute.String("payment.transaction_id", transactionID),
	)

	transaction, err := GetTransaction(transactionID)
	if err != nil {
		correlationID := common.GetCorrelationID(r.Context())
		common.WriteErrorResponse(r.Context(), w, common.ErrNotFound, http.StatusNotFound, correlationID)
		return
	}

	slog.InfoContext(r.Context(), "transaction queried",
		"transaction_id", transactionID,
		"order_id", transaction.OrderID,
		"status", transaction.Status,
		"processor", transaction.Processor,
	)

	// Create response
	response := ChargeResponse{
		TransactionID: transaction.ID,
		OrderID:       transaction.OrderID,
		Amount:        transaction.Amount,
		Status:        transaction.Status,
		Processor:     transaction.Processor,
		Message:       transaction.Message,
		CreatedAt:     transaction.CreatedAt,
	}

	if err := common.WriteJSONResponse(w, response, http.StatusOK); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write transaction response", "error", err)
	}
}
