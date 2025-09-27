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

	if health.Service != "shipping-service" {
		t.Errorf("Expected service 'shipping-service', got %s", health.Service)
	}
}

func TestGetRatesHandler(t *testing.T) {
	if err := LoadCarrierConfig("config.yaml"); err != nil {
		t.Fatalf("Failed to load carrier config: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/shipping/rates", nil)
	rec := httptest.NewRecorder()

	GetRatesHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response RatesResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(response.Carriers) != 3 {
		t.Errorf("Expected 3 carriers, got %d", len(response.Carriers))
	}

	expectedCarriers := map[string]float64{
		"ponyexpress": 9.99,
		"avianair":    19.99,
		"catcarrier":  14.99,
	}

	for _, carrier := range response.Carriers {
		expectedRate, exists := expectedCarriers[carrier.ID]
		if !exists {
			t.Errorf("Unexpected carrier ID: %s", carrier.ID)
			continue
		}
		if carrier.Rate != expectedRate {
			t.Errorf("Expected rate %.2f for %s, got %.2f",
				expectedRate, carrier.ID, carrier.Rate)
		}
	}
}

func TestCreateShipmentHandler(t *testing.T) {
	InitShipmentStorage()
	if err := LoadCarrierConfig("config.yaml"); err != nil {
		t.Fatalf("Failed to load carrier config: %v", err)
	}

	shipReq := ShipRequest{
		OrderID: "test-order-456",
		Carrier: "ponyexpress",
	}
	body, _ := json.Marshal(shipReq)

	req := httptest.NewRequest("POST", "/api/shipping/ship", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	CreateShipmentHandler(rec, req)

	var response ShipResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.OrderID != "test-order-456" {
		t.Errorf("Expected order ID 'test-order-456', got %s", response.OrderID)
	}

	if response.Carrier != "ponyexpress" {
		t.Errorf("Expected carrier 'ponyexpress', got %s", response.Carrier)
	}

	if response.ShipmentID == "" {
		t.Error("Expected shipment ID to be set")
	}

	if response.Status != "shipped" && response.Status != "failed" {
		t.Errorf("Expected status to be 'shipped' or 'failed', got %s", response.Status)
	}

	if response.Cost != 9.99 {
		t.Errorf("Expected cost 9.99, got %.2f", response.Cost)
	}
}

func TestCreateShipmentHandlerInvalidRequest(t *testing.T) {
	InitShipmentStorage()
	if err := LoadCarrierConfig("config.yaml"); err != nil {
		t.Fatalf("Failed to load carrier config: %v", err)
	}

	shipReq := ShipRequest{
		OrderID: "",
	}
	body, _ := json.Marshal(shipReq)

	req := httptest.NewRequest("POST", "/api/shipping/ship", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	CreateShipmentHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestGetShipmentHandler(t *testing.T) {
	InitShipmentStorage()

	testShipment := &Shipment{
		ID:          "test-ship-123",
		OrderID:     "order-789",
		Carrier:     "avianair",
		CarrierName: "Avian Air Express",
		TrackingNum: "AA123456",
		Status:      "shipped",
		Cost:        19.99,
	}

	shipmentsMutex.Lock()
	shipments[testShipment.ID] = testShipment
	shipmentsMutex.Unlock()

	req := httptest.NewRequest("GET", "/api/shipping/test-ship-123", nil)
	rec := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/shipping/{id}", GetShipmentHandler)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response ShipResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.ShipmentID != "test-ship-123" {
		t.Errorf("Expected shipment ID 'test-ship-123', got %s", response.ShipmentID)
	}

	if response.OrderID != "order-789" {
		t.Errorf("Expected order ID 'order-789', got %s", response.OrderID)
	}

	if response.TrackingNum != "AA123456" {
		t.Errorf("Expected tracking number 'AA123456', got %s", response.TrackingNum)
	}

	if response.Cost != 19.99 {
		t.Errorf("Expected cost 19.99, got %.2f", response.Cost)
	}
}

func TestGetShipmentHandlerNotFound(t *testing.T) {
	InitShipmentStorage()

	req := httptest.NewRequest("GET", "/api/shipping/nonexistent", nil)
	rec := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/shipping/{id}", GetShipmentHandler)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}
