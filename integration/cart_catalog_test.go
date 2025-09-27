package integration

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/griffincommerce/demo/common"
)

// Test data
var testProducts = []common.Product{
	{
		ID:       "PROD-INT-001",
		Name:     "Integration Test Dog Toy",
		Price:    15.99,
		Category: "Toys",
		Stock:    100,
	},
	{
		ID:       "PROD-INT-002",
		Name:     "Integration Test Dog Food",
		Price:    45.50,
		Category: "Food",
		Stock:    50,
	},
	{
		ID:       "PROD-INT-003",
		Name:     "Out of Stock Item",
		Price:    25.00,
		Category: "Toys",
		Stock:    0,
	},
}

func TestCartCatalogIntegration(t *testing.T) {
	// Start catalog service with test data
	catalogRouter := mux.NewRouter()
	setupCatalogService(t, catalogRouter)
	catalogServer := httptest.NewServer(catalogRouter)
	defer catalogServer.Close()

	// Start cart service pointing to catalog service
	cartRouter := mux.NewRouter()
	setupCartService(cartRouter, catalogServer.URL)
	cartServer := httptest.NewServer(cartRouter)
	defer cartServer.Close()

	// Run integration tests
	t.Run("HealthCheck", func(t *testing.T) {
		testHealthEndpoints(t, catalogServer.URL, cartServer.URL)
	})

	t.Run("CreateCartAndAddProduct", func(t *testing.T) {
		testCreateCartAndAddProduct(t, cartServer.URL)
	})

	t.Run("AddNonExistentProduct", func(t *testing.T) {
		testAddNonExistentProduct(t, cartServer.URL)
	})

	t.Run("MultipleProductsTotal", func(t *testing.T) {
		testMultipleProductsTotal(t, cartServer.URL)
	})

	t.Run("RemoveProductFromCart", func(t *testing.T) {
		testRemoveProduct(t, cartServer.URL)
	})

	t.Run("ClearCart", func(t *testing.T) {
		testClearCart(t, cartServer.URL)
	})
}

func setupCatalogService(t *testing.T, router *mux.Router) {
	// Initialize a simple in-memory product database
	productDB := common.NewMockDB()

	// Add test products (store as value, not pointer)
	for _, product := range testProducts {
		p := product // Create a copy
		if err := productDB.Set(p.ID, p); err != nil {
			t.Fatalf("Failed to add test product: %v", err)
		}
	}

	// Register catalog routes
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		health := common.HealthResponse{
			Status:  "healthy",
			Service: "catalog-service",
		}
		if err := common.WriteJSONResponse(w, health, http.StatusOK); err != nil {
			slog.Error("Failed to write response", "error", err)
		}
	}).Methods("GET")

	router.HandleFunc("/api/products", func(w http.ResponseWriter, r *http.Request) {
		products := append([]common.Product{}, testProducts...)
		if err := common.WriteJSONResponse(w, products, http.StatusOK); err != nil {
			slog.Error("Failed to write response", "error", err)
		}
	}).Methods("GET")

	router.HandleFunc("/api/products/{id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		productID := vars["id"]

		data, err := productDB.Get(productID)
		if err != nil {
			common.WriteErrorResponse(w, common.ErrNotFound, http.StatusNotFound, "")
			return
		}

		product, ok := data.(common.Product)
		if !ok {
			common.WriteErrorResponse(w, common.NewAppError("INTERNAL_ERROR", "Internal server error"), http.StatusInternalServerError, "")
			return
		}

		if err := common.WriteJSONResponse(w, product, http.StatusOK); err != nil {
			slog.Error("Failed to write response", "error", err)
		}
	}).Methods("GET")
}

func setupCartService(router *mux.Router, catalogURL string) {
	// Initialize cart database
	cartDB := common.NewMockDB()

	// Simple HTTP client for product service
	httpClient := &http.Client{}

	// Helper function to get product from catalog
	getProduct := func(productID string) (*common.Product, error) {
		resp, err := httpClient.Get(catalogURL + "/api/products/" + productID)
		if err != nil {
			return nil, err
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				slog.Warn("Failed to close response body", "error", err)
			}
		}()

		if resp.StatusCode == http.StatusNotFound {
			return nil, common.ErrNotFound
		}

		var product common.Product
		if err := json.NewDecoder(resp.Body).Decode(&product); err != nil {
			return nil, err
		}

		return &product, nil
	}

	// Register cart routes
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		health := common.HealthResponse{
			Status:  "healthy",
			Service: "cart-service",
		}
		if err := common.WriteJSONResponse(w, health, http.StatusOK); err != nil {
			slog.Error("Failed to write health response", "error", err)
		}
	}).Methods("GET")

	router.HandleFunc("/api/cart/create", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			CustomerID string `json:"customer_id"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteErrorResponse(w, common.ErrBadRequest, http.StatusBadRequest, "")
			return
		}

		cart := common.Cart{
			ID:         "CART-" + req.CustomerID,
			CustomerID: req.CustomerID,
			Items:      []common.CartItem{},
			Total:      0,
		}

		if err := cartDB.Set(cart.ID, cart); err != nil {
			common.WriteErrorResponse(w, common.NewAppError("INTERNAL_ERROR", "Internal server error"), http.StatusInternalServerError, "")
			return
		}

		if err := common.WriteJSONResponse(w, cart, http.StatusCreated); err != nil {
			slog.Error("Failed to write cart response", "error", err)
		}
	}).Methods("POST")

	router.HandleFunc("/api/cart/{id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		cartID := vars["id"]

		data, err := cartDB.Get(cartID)
		if err != nil {
			common.WriteErrorResponse(w, common.ErrNotFound, http.StatusNotFound, "")
			return
		}

		cartData, ok := data.(common.Cart)
		if !ok {
			common.WriteErrorResponse(w, common.NewAppError("INTERNAL_ERROR", "Internal server error"), http.StatusInternalServerError, "")
			return
		}
		cart := cartData

		if err := common.WriteJSONResponse(w, cart, http.StatusOK); err != nil {
			slog.Error("Failed to write cart response", "error", err)
		}
	}).Methods("GET")

	router.HandleFunc("/api/cart/{id}/add", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		cartID := vars["id"]

		var req struct {
			ProductID string `json:"product_id"`
			Quantity  int    `json:"quantity"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteErrorResponse(w, common.ErrBadRequest, http.StatusBadRequest, "")
			return
		}

		if req.ProductID == "" || req.Quantity <= 0 {
			common.WriteErrorResponse(w, common.ErrBadRequest, http.StatusBadRequest, "")
			return
		}

		// Get cart
		data, err := cartDB.Get(cartID)
		if err != nil {
			common.WriteErrorResponse(w, common.ErrNotFound, http.StatusNotFound, "")
			return
		}

		cartData, ok := data.(common.Cart)
		if !ok {
			common.WriteErrorResponse(w, common.NewAppError("INTERNAL_ERROR", "Internal server error"), http.StatusInternalServerError, "")
			return
		}
		cart := cartData

		// Get product from catalog service
		product, err := getProduct(req.ProductID)
		if err != nil {
			if err == common.ErrNotFound {
				common.WriteErrorResponse(w, common.ErrNotFound, http.StatusNotFound, "")
			} else {
				common.WriteErrorResponse(w, common.NewAppError("INTERNAL_ERROR", "Internal server error"), http.StatusInternalServerError, "")
			}
			return
		}

		// Add item to cart
		item := common.CartItem{
			ProductID: product.ID,
			Name:      product.Name,
			Price:     product.Price,
			Quantity:  req.Quantity,
			Subtotal:  product.Price * float64(req.Quantity),
		}

		// Check if item already exists
		found := false
		for i, existingItem := range cart.Items {
			if existingItem.ProductID == product.ID {
				cart.Items[i].Quantity += req.Quantity
				cart.Items[i].Subtotal = cart.Items[i].Price * float64(cart.Items[i].Quantity)
				found = true
				break
			}
		}

		if !found {
			cart.Items = append(cart.Items, item)
		}

		// Update total
		cart.Total = 0
		for _, item := range cart.Items {
			cart.Total += item.Subtotal
		}

		// Save cart
		if err := cartDB.Set(cart.ID, cart); err != nil {
			common.WriteErrorResponse(w, common.NewAppError("INTERNAL_ERROR", "Internal server error"), http.StatusInternalServerError, "")
			return
		}

		if err := common.WriteJSONResponse(w, cart, http.StatusOK); err != nil {
			slog.Error("Failed to write cart response", "error", err)
		}
	}).Methods("POST")

	router.HandleFunc("/api/cart/{id}/item/{productId}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		cartID := vars["id"]
		productID := vars["productId"]

		// Get cart
		data, err := cartDB.Get(cartID)
		if err != nil {
			common.WriteErrorResponse(w, common.ErrNotFound, http.StatusNotFound, "")
			return
		}

		cartData, ok := data.(common.Cart)
		if !ok {
			common.WriteErrorResponse(w, common.NewAppError("INTERNAL_ERROR", "Internal server error"), http.StatusInternalServerError, "")
			return
		}
		cart := cartData

		// Remove item
		newItems := []common.CartItem{}
		for _, item := range cart.Items {
			if item.ProductID != productID {
				newItems = append(newItems, item)
			}
		}
		cart.Items = newItems

		// Update total
		cart.Total = 0
		for _, item := range cart.Items {
			cart.Total += item.Subtotal
		}

		// Save cart
		if err := cartDB.Set(cart.ID, cart); err != nil {
			common.WriteErrorResponse(w, common.NewAppError("INTERNAL_ERROR", "Internal server error"), http.StatusInternalServerError, "")
			return
		}

		if err := common.WriteJSONResponse(w, cart, http.StatusOK); err != nil {
			slog.Error("Failed to write cart response", "error", err)
		}
	}).Methods("DELETE")

	router.HandleFunc("/api/cart/{id}/clear", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		cartID := vars["id"]

		// Get cart
		data, err := cartDB.Get(cartID)
		if err != nil {
			common.WriteErrorResponse(w, common.ErrNotFound, http.StatusNotFound, "")
			return
		}

		cartData, ok := data.(common.Cart)
		if !ok {
			common.WriteErrorResponse(w, common.NewAppError("INTERNAL_ERROR", "Internal server error"), http.StatusInternalServerError, "")
			return
		}
		cart := cartData

		// Clear cart
		cart.Items = []common.CartItem{}
		cart.Total = 0

		// Save cart
		if err := cartDB.Set(cart.ID, cart); err != nil {
			common.WriteErrorResponse(w, common.NewAppError("INTERNAL_ERROR", "Internal server error"), http.StatusInternalServerError, "")
			return
		}

		if err := common.WriteJSONResponse(w, cart, http.StatusOK); err != nil {
			slog.Error("Failed to write cart response", "error", err)
		}
	}).Methods("DELETE")
}

// Test functions
func testHealthEndpoints(t *testing.T, catalogURL, cartURL string) {
	// Check catalog health
	resp, err := http.Get(catalogURL + "/health")
	if err != nil {
		t.Fatalf("Failed to check catalog health: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Catalog health check failed: expected 200, got %d", resp.StatusCode)
	}

	// Check cart health
	resp, err = http.Get(cartURL + "/health")
	if err != nil {
		t.Fatalf("Failed to check cart health: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Cart health check failed: expected 200, got %d", resp.StatusCode)
	}
}

func testCreateCartAndAddProduct(t *testing.T, cartURL string) {
	// Create cart
	createReq := struct {
		CustomerID string `json:"customer_id"`
	}{CustomerID: "test-customer-1"}
	body, _ := json.Marshal(createReq)

	resp, err := http.Post(cartURL+"/api/cart/create", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Failed to create cart: %v", err)
	}

	var cart common.Cart
	if err := json.NewDecoder(resp.Body).Decode(&cart); err != nil {
		t.Fatalf("Failed to decode cart response: %v", err)
	}

	// Add product
	addReq := struct {
		ProductID string `json:"product_id"`
		Quantity  int    `json:"quantity"`
	}{
		ProductID: "PROD-INT-001",
		Quantity:  2,
	}
	body, _ = json.Marshal(addReq)

	resp, err = http.Post(cartURL+"/api/cart/"+cart.ID+"/add", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Failed to add item: %v", err)
	}

	if err := json.NewDecoder(resp.Body).Decode(&cart); err != nil {
		t.Fatalf("Failed to decode cart response: %v", err)
	}

	if len(cart.Items) != 1 {
		t.Errorf("Expected 1 item, got %d", len(cart.Items))
	}

	if cart.Items[0].Name != "Integration Test Dog Toy" {
		t.Errorf("Product name mismatch: got %s", cart.Items[0].Name)
	}

	expectedTotal := 15.99 * 2
	if cart.Total != expectedTotal {
		t.Errorf("Expected total %.2f, got %.2f", expectedTotal, cart.Total)
	}
}

func testAddNonExistentProduct(t *testing.T, cartURL string) {
	// Create cart
	createReq := struct {
		CustomerID string `json:"customer_id"`
	}{CustomerID: "test-customer-2"}
	body, _ := json.Marshal(createReq)

	resp, _ := http.Post(cartURL+"/api/cart/create", "application/json", bytes.NewBuffer(body))

	var cart common.Cart
	if err := json.NewDecoder(resp.Body).Decode(&cart); err != nil {
		t.Fatalf("Failed to decode cart response: %v", err)
	}

	// Try to add non-existent product
	addReq := struct {
		ProductID string `json:"product_id"`
		Quantity  int    `json:"quantity"`
	}{
		ProductID: "DOES-NOT-EXIST",
		Quantity:  1,
	}
	body, _ = json.Marshal(addReq)

	resp, _ = http.Post(cartURL+"/api/cart/"+cart.ID+"/add", "application/json", bytes.NewBuffer(body))

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 for non-existent product, got %d", resp.StatusCode)
	}
}

func testMultipleProductsTotal(t *testing.T, cartURL string) {
	// Create cart
	createReq := struct {
		CustomerID string `json:"customer_id"`
	}{CustomerID: "test-customer-3"}
	body, _ := json.Marshal(createReq)

	resp, _ := http.Post(cartURL+"/api/cart/create", "application/json", bytes.NewBuffer(body))

	var cart common.Cart
	if err := json.NewDecoder(resp.Body).Decode(&cart); err != nil {
		t.Fatalf("Failed to decode cart response: %v", err)
	}

	// Add first product (2 x $15.99 = $31.98)
	addReq := struct {
		ProductID string `json:"product_id"`
		Quantity  int    `json:"quantity"`
	}{
		ProductID: "PROD-INT-001",
		Quantity:  2,
	}
	body, _ = json.Marshal(addReq)
	if _, err := http.Post(cartURL+"/api/cart/"+cart.ID+"/add", "application/json", bytes.NewBuffer(body)); err != nil {
		t.Fatalf("Failed to add product: %v", err)
	}

	// Add second product (1 x $45.50 = $45.50)
	addReq.ProductID = "PROD-INT-002"
	addReq.Quantity = 1
	body, _ = json.Marshal(addReq)
	if resp, err := http.Post(cartURL+"/api/cart/"+cart.ID+"/add", "application/json", bytes.NewBuffer(body)); err != nil {
		t.Fatalf("Failed to add product: %v", err)
	} else {
		if err := json.NewDecoder(resp.Body).Decode(&cart); err != nil {
			t.Fatalf("Failed to decode cart response: %v", err)
		}
	}

	expectedTotal := (2 * 15.99) + (1 * 45.50) // $77.48
	if cart.Total != expectedTotal {
		t.Errorf("Expected total %.2f, got %.2f", expectedTotal, cart.Total)
	}
}

func testRemoveProduct(t *testing.T, cartURL string) {
	// Create cart with products
	createReq := struct {
		CustomerID string `json:"customer_id"`
	}{CustomerID: "test-customer-4"}
	body, _ := json.Marshal(createReq)

	resp, _ := http.Post(cartURL+"/api/cart/create", "application/json", bytes.NewBuffer(body))

	var cart common.Cart
	if err := json.NewDecoder(resp.Body).Decode(&cart); err != nil {
		t.Fatalf("Failed to decode cart response: %v", err)
	}

	// Add two products
	for _, p := range []string{"PROD-INT-001", "PROD-INT-002"} {
		addReq := struct {
			ProductID string `json:"product_id"`
			Quantity  int    `json:"quantity"`
		}{
			ProductID: p,
			Quantity:  1,
		}
		body, _ = json.Marshal(addReq)
		if _, err := http.Post(cartURL+"/api/cart/"+cart.ID+"/add", "application/json", bytes.NewBuffer(body)); err != nil {
			t.Fatalf("Failed to add product: %v", err)
		}
	}

	// Remove first product
	req, _ := http.NewRequest("DELETE", cartURL+"/api/cart/"+cart.ID+"/item/PROD-INT-001", nil)
	client := &http.Client{}
	resp, _ = client.Do(req)

	if err := json.NewDecoder(resp.Body).Decode(&cart); err != nil {
		t.Fatalf("Failed to decode cart response: %v", err)
	}

	if len(cart.Items) != 1 {
		t.Errorf("Expected 1 item after removal, got %d", len(cart.Items))
	}

	if cart.Items[0].ProductID != "PROD-INT-002" {
		t.Errorf("Wrong product remaining")
	}
}

func testClearCart(t *testing.T, cartURL string) {
	// Create cart with products
	createReq := struct {
		CustomerID string `json:"customer_id"`
	}{CustomerID: "test-customer-5"}
	body, _ := json.Marshal(createReq)

	resp, _ := http.Post(cartURL+"/api/cart/create", "application/json", bytes.NewBuffer(body))

	var cart common.Cart
	if err := json.NewDecoder(resp.Body).Decode(&cart); err != nil {
		t.Fatalf("Failed to decode cart response: %v", err)
	}

	// Add a product
	addReq := struct {
		ProductID string `json:"product_id"`
		Quantity  int    `json:"quantity"`
	}{
		ProductID: "PROD-INT-001",
		Quantity:  3,
	}
	body, _ = json.Marshal(addReq)
	if _, err := http.Post(cartURL+"/api/cart/"+cart.ID+"/add", "application/json", bytes.NewBuffer(body)); err != nil {
		t.Fatalf("Failed to add product: %v", err)
	}

	// Clear cart
	req, _ := http.NewRequest("DELETE", cartURL+"/api/cart/"+cart.ID+"/clear", nil)
	client := &http.Client{}
	resp, _ = client.Do(req)

	if err := json.NewDecoder(resp.Body).Decode(&cart); err != nil {
		t.Fatalf("Failed to decode cart response: %v", err)
	}

	if len(cart.Items) != 0 {
		t.Errorf("Expected empty cart, got %d items", len(cart.Items))
	}

	if cart.Total != 0 {
		t.Errorf("Expected total 0, got %.2f", cart.Total)
	}
}
