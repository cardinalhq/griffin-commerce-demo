package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/cardinalhq/griffin-commerce-demo/common"
)

// RegisterRoutes registers all HTTP routes
func RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/health", HealthHandler).Methods("GET")
	r.HandleFunc("/api/payments/charge", ChargeHandler).Methods("POST", "OPTIONS")
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
		slog.Error("Failed to write health response", "error", err)
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
		common.WriteErrorResponse(w, common.ErrBadRequest, http.StatusBadRequest, correlationID)
		return
	}

	// Validate request
	if req.OrderID == "" || req.Amount <= 0 {
		correlationID := common.GetCorrelationID(r.Context())
		err := common.NewAppError("INVALID_REQUEST", "Order ID and positive amount are required")
		common.WriteErrorResponse(w, err, http.StatusBadRequest, correlationID)
		return
	}

	// Process payment
	transaction, err := ProcessPayment(req.OrderID, req.Amount, req.Processor)
	if err != nil {
		correlationID := common.GetCorrelationID(r.Context())
		appErr := common.NewAppError("PROCESSING_ERROR", err.Error())
		common.WriteErrorResponse(w, appErr, http.StatusInternalServerError, correlationID)
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

	if err := common.WriteJSONResponse(w, response, statusCode); err != nil {
		slog.Error("Failed to write charge response", "error", err, "status", statusCode)
	}
}

// GetTransactionHandler retrieves a transaction by ID
func GetTransactionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	transactionID := vars["id"]

	transaction, err := GetTransaction(transactionID)
	if err != nil {
		correlationID := common.GetCorrelationID(r.Context())
		common.WriteErrorResponse(w, common.ErrNotFound, http.StatusNotFound, correlationID)
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

	if err := common.WriteJSONResponse(w, response, http.StatusOK); err != nil {
		slog.Error("Failed to write transaction response", "error", err)
	}
}
