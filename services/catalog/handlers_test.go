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

func init() {
	productDB = common.NewMockDB()
}

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

	if health.Service != "catalog-service" {
		t.Errorf("Expected service 'catalog-service', got %s", health.Service)
	}
}

func TestGetProductsHandler(t *testing.T) {
	productDB = common.NewMockDB()
	testProducts := []common.Product{
		{ID: "test1", Name: "Test Product 1", Price: 10.00, Stock: 100},
		{ID: "test2", Name: "Test Product 2", Price: 20.00, Stock: 50},
	}
	for _, p := range testProducts {
		productDB.Set(p.ID, p)
	}

	req := httptest.NewRequest("GET", "/api/products", nil)
	rec := httptest.NewRecorder()

	GetProductsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var products []common.Product
	if err := json.NewDecoder(rec.Body).Decode(&products); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(products) != 2 {
		t.Errorf("Expected 2 products, got %d", len(products))
	}
}

func TestGetProductHandler(t *testing.T) {
	productDB = common.NewMockDB()
	testProduct := common.Product{
		ID:    "test123",
		Name:  "Test Product",
		Price: 15.99,
		Stock: 75,
	}
	productDB.Set(testProduct.ID, testProduct)

	req := httptest.NewRequest("GET", "/api/products/test123", nil)
	rec := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/products/{id}", GetProductHandler)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var product common.Product
	if err := json.NewDecoder(rec.Body).Decode(&product); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if product.ID != testProduct.ID {
		t.Errorf("Expected product ID %s, got %s", testProduct.ID, product.ID)
	}

	if product.Price != testProduct.Price {
		t.Errorf("Expected price %.2f, got %.2f", testProduct.Price, product.Price)
	}
}

func TestReserveStockHandler(t *testing.T) {
	productDB = common.NewMockDB()
	testProduct := common.Product{
		ID:    "stock-test",
		Name:  "Stock Test Product",
		Price: 25.00,
		Stock: 100,
	}
	productDB.Set(testProduct.ID, testProduct)

	stockRequest := StockRequest{
		Quantity: 10,
	}
	body, _ := json.Marshal(stockRequest)

	req := httptest.NewRequest("POST", "/api/products/stock-test/reserve", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/products/{id}/reserve", ReserveStockHandler)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response StockResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.ProductID != "stock-test" {
		t.Errorf("Expected product ID 'stock-test', got %s", response.ProductID)
	}

	if response.Stock != 90 {
		t.Errorf("Expected stock 90 after reservation, got %d", response.Stock)
	}

	product, _ := GetProduct("stock-test")
	if product.Stock != 90 {
		t.Errorf("Expected stock to be 90, got %d", product.Stock)
	}
}

func TestReleaseStockHandler(t *testing.T) {
	productDB = common.NewMockDB()
	testProduct := common.Product{
		ID:    "release-test",
		Name:  "Release Test Product",
		Price: 30.00,
		Stock: 50,
	}
	productDB.Set(testProduct.ID, testProduct)

	stockRequest := StockRequest{
		Quantity: 20,
	}
	body, _ := json.Marshal(stockRequest)

	req := httptest.NewRequest("POST", "/api/products/release-test/release", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/products/{id}/release", ReleaseStockHandler)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response StockResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.ProductID != "release-test" {
		t.Errorf("Expected product ID 'release-test', got %s", response.ProductID)
	}

	if response.Stock != 70 {
		t.Errorf("Expected stock 70 after release, got %d", response.Stock)
	}

	product, _ := GetProduct("release-test")
	if product.Stock != 70 {
		t.Errorf("Expected stock to be 70, got %d", product.Stock)
	}
}