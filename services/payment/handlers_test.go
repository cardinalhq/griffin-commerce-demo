package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/griffincommerce/demo/common"
)

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	HealthHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var health common.HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if health.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got %s", health.Status)
	}

	if health.Service != "payment-service" {
		t.Errorf("Expected service 'payment-service', got %s", health.Service)
	}
}

func TestChargeHandler(t *testing.T) {
	InitTransactionStorage()
	LoadProcessorConfig("config.yaml")

	chargeReq := ChargeRequest{
		OrderID:   "test-order-123",
		Amount:    99.99,
		Processor: "puppypay",
	}
	body, _ := json.Marshal(chargeReq)

	req := httptest.NewRequest("POST", "/api/payments/charge", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	ChargeHandler(rec, req)

	var response ChargeResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.OrderID != "test-order-123" {
		t.Errorf("Expected order ID 'test-order-123', got %s", response.OrderID)
	}

	if response.Amount != 99.99 {
		t.Errorf("Expected amount 99.99, got %.2f", response.Amount)
	}

	if response.TransactionID == "" {
		t.Error("Expected transaction ID to be set")
	}

	if response.Status != "success" && response.Status != "failed" {
		t.Errorf("Expected status to be 'success' or 'failed', got %s", response.Status)
	}

	if response.Processor != "PuppyPay" {
		t.Errorf("Expected processor 'PuppyPay', got %s", response.Processor)
	}
}

func TestChargeHandlerInvalidRequest(t *testing.T) {
	InitTransactionStorage()

	tests := []struct {
		name string
		req  ChargeRequest
	}{
		{
			name: "missing order ID",
			req:  ChargeRequest{Amount: 50.00},
		},
		{
			name: "zero amount",
			req:  ChargeRequest{OrderID: "test", Amount: 0},
		},
		{
			name: "negative amount",
			req:  ChargeRequest{OrderID: "test", Amount: -10.00},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body, _ := json.Marshal(test.req)
			req := httptest.NewRequest("POST", "/api/payments/charge", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			ChargeHandler(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("%s: Expected status %d, got %d", test.name, http.StatusBadRequest, rec.Code)
			}
		})
	}
}

func TestGetTransactionHandler(t *testing.T) {
	InitTransactionStorage()
	LoadProcessorConfig("config.yaml")

	transaction, _ := ProcessPayment("order-123", 75.50, "kittycard")
	transactionID := transaction.ID

	req := httptest.NewRequest("GET", "/api/payments/"+transactionID, nil)
	rec := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/payments/{id}", GetTransactionHandler)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response ChargeResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.TransactionID != transactionID {
		t.Errorf("Expected transaction ID '%s', got %s", transactionID, response.TransactionID)
	}

	if response.OrderID != "order-123" {
		t.Errorf("Expected order ID 'order-123', got %s", response.OrderID)
	}

	if response.Amount != 75.50 {
		t.Errorf("Expected amount 75.50, got %.2f", response.Amount)
	}

	// Transaction can be either success or failed due to random failure rates
	if response.Status != "success" && response.Status != "failed" {
		t.Errorf("Expected status 'success' or 'failed', got %s", response.Status)
	}
}

func TestGetTransactionHandlerNotFound(t *testing.T) {
	InitTransactionStorage()

	req := httptest.NewRequest("GET", "/api/payments/nonexistent", nil)
	rec := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/payments/{id}", GetTransactionHandler)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}
