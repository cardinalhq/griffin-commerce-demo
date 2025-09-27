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
	// Initialize cart storage for tests
	InitCartStorage()
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

	if health.Service != "cart-service" {
		t.Errorf("Expected service 'cart-service', got %s", health.Service)
	}
}

func TestCreateCartHandler(t *testing.T) {
	createReq := CreateCartRequest{CustomerID: "test-customer-123"}
	body, _ := json.Marshal(createReq)

	req := httptest.NewRequest("POST", "/api/cart/create", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	CreateCartHandler(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var cart common.Cart
	if err := json.NewDecoder(rec.Body).Decode(&cart); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if cart.CustomerID != "test-customer-123" {
		t.Errorf("Expected customer ID 'test-customer-123', got %s", cart.CustomerID)
	}

	if cart.ID == "" {
		t.Error("Expected cart to have an ID")
	}

	if len(cart.Items) != 0 {
		t.Errorf("Expected empty cart, got %d items", len(cart.Items))
	}
}

func TestGetCartHandler(t *testing.T) {
	// Create a cart first
	cart, err := CreateCart("test-customer")
	if err != nil {
		t.Fatalf("Failed to create cart: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/cart/"+cart.ID, nil)
	rec := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/cart/{id}", GetCartHandler)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var retrievedCart common.Cart
	if err := json.NewDecoder(rec.Body).Decode(&retrievedCart); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if retrievedCart.ID != cart.ID {
		t.Errorf("Expected cart ID %s, got %s", cart.ID, retrievedCart.ID)
	}

	if retrievedCart.CustomerID != "test-customer" {
		t.Errorf("Expected customer ID 'test-customer', got %s", retrievedCart.CustomerID)
	}
}

func TestAddItemHandler(t *testing.T) {
	// Mock product service
	productServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		product := common.Product{
			ID:    "test-product",
			Name:  "Test Product",
			Price: 19.99,
			Stock: 100,
		}
		if err := json.NewEncoder(w).Encode(product); err != nil {
			t.Errorf("Failed to encode product: %v", err)
		}
	}))
	defer productServer.Close()

	// Initialize the product client with mock server URL
	InitProductClient(productServer.URL)

	// Create a cart
	cart, err := CreateCart("test-customer")
	if err != nil {
		t.Fatalf("Failed to create cart: %v", err)
	}

	addItemReq := AddItemRequest{
		ProductID: "test-product",
		Quantity:  2,
	}
	body, _ := json.Marshal(addItemReq)

	req := httptest.NewRequest("POST", "/api/cart/"+cart.ID+"/add", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/cart/{id}/add", AddItemHandler)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var updatedCart common.Cart
	if err := json.NewDecoder(rec.Body).Decode(&updatedCart); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(updatedCart.Items) != 1 {
		t.Errorf("Expected 1 item in cart, got %d", len(updatedCart.Items))
	}

	if updatedCart.Items[0].Quantity != 2 {
		t.Errorf("Expected quantity 2, got %d", updatedCart.Items[0].Quantity)
	}

	expectedTotal := 19.99 * 2
	if updatedCart.Total != expectedTotal {
		t.Errorf("Expected total %.2f, got %.2f", expectedTotal, updatedCart.Total)
	}
}

func TestRemoveItemHandler(t *testing.T) {
	// Create a cart
	cart, err := CreateCart("test-customer")
	if err != nil {
		t.Fatalf("Failed to create cart: %v", err)
	}

	// Manually add items to the cart
	cart.Items = []common.CartItem{
		{
			ProductID: "product1",
			Name:      "Product 1",
			Price:     10.00,
			Quantity:  1,
			Subtotal:  10.00,
		},
		{
			ProductID: "product2",
			Name:      "Product 2",
			Price:     20.00,
			Quantity:  2,
			Subtotal:  40.00,
		},
	}
	cart.Total = 50.00

	// Save the updated cart
	if err := cartDB.Set(cart.ID, cart); err != nil {
		t.Fatalf("Failed to update cart: %v", err)
	}

	req := httptest.NewRequest("DELETE", "/api/cart/"+cart.ID+"/item/product1", nil)
	rec := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/cart/{id}/item/{productId}", RemoveItemHandler)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var updatedCart common.Cart
	if err := json.NewDecoder(rec.Body).Decode(&updatedCart); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(updatedCart.Items) != 1 {
		t.Errorf("Expected 1 item in cart after removal, got %d", len(updatedCart.Items))
	}

	if updatedCart.Items[0].ProductID != "product2" {
		t.Errorf("Expected remaining product to be 'product2', got %s", updatedCart.Items[0].ProductID)
	}

	if updatedCart.Total != 40.00 {
		t.Errorf("Expected total 40.00 after removal, got %.2f", updatedCart.Total)
	}
}

func TestClearCartHandler(t *testing.T) {
	// Create a cart
	cart, err := CreateCart("test-customer")
	if err != nil {
		t.Fatalf("Failed to create cart: %v", err)
	}

	// Add items to the cart
	cart.Items = []common.CartItem{
		{
			ProductID: "product1",
			Name:      "Product 1",
			Price:     10.00,
			Quantity:  2,
			Subtotal:  20.00,
		},
	}
	cart.Total = 20.00

	// Save the updated cart
	if err := cartDB.Set(cart.ID, cart); err != nil {
		t.Fatalf("Failed to update cart: %v", err)
	}

	req := httptest.NewRequest("DELETE", "/api/cart/"+cart.ID+"/clear", nil)
	rec := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/cart/{id}/clear", ClearCartHandler)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var clearedCart common.Cart
	if err := json.NewDecoder(rec.Body).Decode(&clearedCart); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(clearedCart.Items) != 0 {
		t.Errorf("Expected empty cart after clearing, got %d items", len(clearedCart.Items))
	}

	if clearedCart.Total != 0 {
		t.Errorf("Expected total 0 after clearing, got %.2f", clearedCart.Total)
	}
}

func TestCheckoutHandler(t *testing.T) {
	// Create a cart with items
	cart, err := CreateCart("test-customer")
	if err != nil {
		t.Fatalf("Failed to create cart: %v", err)
	}

	cart.Items = []common.CartItem{
		{
			ProductID: "product1",
			Name:      "Product 1",
			Price:     25.00,
			Quantity:  2,
			Subtotal:  50.00,
		},
	}
	cart.Total = 50.00

	// Save the updated cart
	if err := cartDB.Set(cart.ID, cart); err != nil {
		t.Fatalf("Failed to update cart: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/cart/"+cart.ID+"/checkout", nil)
	rec := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/api/cart/{id}/checkout", CheckoutHandler)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response CheckoutResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.CartID != cart.ID {
		t.Errorf("Expected cart ID %s, got %s", cart.ID, response.CartID)
	}

	if response.Total != 50.00 {
		t.Errorf("Expected total 50.00, got %.2f", response.Total)
	}

	if response.ItemCount != 1 {
		t.Errorf("Expected 1 item, got %d", response.ItemCount)
	}
}
