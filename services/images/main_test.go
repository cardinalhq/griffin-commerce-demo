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

	if health.Service != "image-service" {
		t.Errorf("Expected service 'image-service', got %s", health.Service)
	}
}

func TestGetProductImageHandler(t *testing.T) {
	tests := []struct {
		productID    string
		expectedPath string
	}{
		{"PROD-001", "/static/rope-toy.jpg"},
		{"PROD-004", "/static/puppy-food.jpg"},
		{"unknown", "/static/placeholder.jpg"},
	}

	for _, test := range tests {
		req := httptest.NewRequest("GET", "/api/images/product/"+test.productID, nil)
		rec := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/api/images/product/{id}", GetProductImageHandler)
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status %d for product %s, got %d",
				http.StatusOK, test.productID, rec.Code)
		}

		var response struct {
			ProductID string `json:"product_id"`
			ImageURL  string `json:"image_url"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response for product %s: %v", test.productID, err)
		}

		if response.ProductID != test.productID {
			t.Errorf("Expected product ID %s, got %s", test.productID, response.ProductID)
		}

		if response.ImageURL != test.expectedPath {
			t.Errorf("Expected image URL %s for product %s, got %s",
				test.expectedPath, test.productID, response.ImageURL)
		}
	}
}

func TestStaticFileServer(t *testing.T) {
	req := httptest.NewRequest("GET", "/static/test.jpg", nil)
	rec := httptest.NewRecorder()

	router := mux.NewRouter()
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/",
		http.FileServer(http.Dir("./static"))))

	router.ServeHTTP(rec, req)

	if rec.Code == http.StatusInternalServerError {
		t.Skip("Static directory not available in test environment")
	}
}
