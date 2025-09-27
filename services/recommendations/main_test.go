package main

import (
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

	if health.Service != "recommendations-service" {
		t.Errorf("Expected service 'recommendations-service', got %s", health.Service)
	}
}

func TestGetRecommendationsHandler(t *testing.T) {
	productCacheMutex.Lock()
	productCache = []common.Product{
		{ID: "prod1", Name: "Product 1", Price: 10.00, Stock: 100},
		{ID: "prod2", Name: "Product 2", Price: 20.00, Stock: 50},
		{ID: "prod3", Name: "Product 3", Price: 30.00, Stock: 25},
		{ID: "prod4", Name: "Product 4", Price: 40.00, Stock: 10},
		{ID: "prod5", Name: "Product 5", Price: 50.00, Stock: 5},
		{ID: "prod6", Name: "Product 6", Price: 60.00, Stock: 15},
	}
	productCacheMutex.Unlock()

	req := httptest.NewRequest("GET", "/api/recommendations", nil)
	rec := httptest.NewRecorder()

	GetRecommendationsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response RecommendationsResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(response.Products) != 5 {
		t.Errorf("Expected 5 recommendations, got %d", len(response.Products))
	}

	if response.Type != "trending" {
		t.Errorf("Expected type 'trending', got %s", response.Type)
	}

	productIDs := make(map[string]bool)
	for _, product := range response.Products {
		if productIDs[product.ID] {
			t.Errorf("Duplicate product ID in recommendations: %s", product.ID)
		}
		productIDs[product.ID] = true
	}
}

func TestGetRecommendationsHandlerWithCount(t *testing.T) {
	productCacheMutex.Lock()
	productCache = []common.Product{
		{ID: "prod1", Name: "Product 1", Price: 10.00, Stock: 100},
		{ID: "prod2", Name: "Product 2", Price: 20.00, Stock: 50},
		{ID: "prod3", Name: "Product 3", Price: 30.00, Stock: 25},
	}
	productCacheMutex.Unlock()

	req := httptest.NewRequest("GET", "/api/recommendations?count=2", nil)
	rec := httptest.NewRecorder()

	GetRecommendationsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response RecommendationsResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(response.Products) != 2 {
		t.Errorf("Expected 2 recommendations, got %d", len(response.Products))
	}
}

func TestGetProductRecommendationsHandler(t *testing.T) {
	productCacheMutex.Lock()
	productCache = []common.Product{
		{ID: "prod1", Name: "Product 1", Price: 10.00, Stock: 100},
		{ID: "prod2", Name: "Product 2", Price: 20.00, Stock: 50},
		{ID: "prod3", Name: "Product 3", Price: 30.00, Stock: 25},
		{ID: "prod4", Name: "Product 4", Price: 40.00, Stock: 10},
		{ID: "prod5", Name: "Product 5", Price: 50.00, Stock: 5},
	}
	productCacheMutex.Unlock()

	req := httptest.NewRequest("GET", "/api/recommendations/product/prod1", nil)
	rec := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/recommendations/product/{id}", GetProductRecommendationsHandler)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response RecommendationsResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(response.Products) != 4 {
		t.Errorf("Expected 4 recommendations (excluding prod1), got %d", len(response.Products))
	}

	if response.Type != "related" {
		t.Errorf("Expected type 'related', got %s", response.Type)
	}

	for _, product := range response.Products {
		if product.ID == "prod1" {
			t.Error("Excluded product ID 'prod1' should not be in recommendations")
		}
	}
}

func TestGetRecommendationsEmptyCache(t *testing.T) {
	productCacheMutex.Lock()
	productCache = []common.Product{}
	productCacheMutex.Unlock()

	req := httptest.NewRequest("GET", "/api/recommendations", nil)
	rec := httptest.NewRecorder()

	GetRecommendationsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response RecommendationsResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(response.Products) != 0 {
		t.Errorf("Expected empty recommendations, got %d", len(response.Products))
	}
}

func TestGetRecommendationsOutOfStock(t *testing.T) {
	productCacheMutex.Lock()
	productCache = []common.Product{
		{ID: "prod1", Name: "Product 1", Price: 10.00, Stock: 0},
		{ID: "prod2", Name: "Product 2", Price: 20.00, Stock: 0},
		{ID: "prod3", Name: "Product 3", Price: 30.00, Stock: 10},
	}
	productCacheMutex.Unlock()

	req := httptest.NewRequest("GET", "/api/recommendations", nil)
	rec := httptest.NewRecorder()

	GetRecommendationsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response RecommendationsResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(response.Products) != 1 {
		t.Errorf("Expected 1 recommendation (only in-stock), got %d", len(response.Products))
	}

	if response.Products[0].ID != "prod3" {
		t.Errorf("Expected only 'prod3' in recommendations, got %s", response.Products[0].ID)
	}
}