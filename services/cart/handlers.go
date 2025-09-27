package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/griffincommerce/demo/common"
)

// RegisterRoutes registers all HTTP routes
func RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/health", HealthHandler).Methods("GET")
	r.HandleFunc("/api/cart/create", CreateCartHandler).Methods("POST")
	r.HandleFunc("/api/cart/{id}", GetCartHandler).Methods("GET")
	r.HandleFunc("/api/cart/{id}/add", AddItemHandler).Methods("POST")
	r.HandleFunc("/api/cart/{id}/item/{productId}", RemoveItemHandler).Methods("DELETE")
	r.HandleFunc("/api/cart/{id}/checkout", CheckoutHandler).Methods("POST")
}

// HealthHandler returns service health status
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	health := common.HealthResponse{
		Status:    "healthy",
		Service:   "cart-service",
		Version:   "1.0.0",
		Timestamp: time.Now(),
	}
	common.WriteJSONResponse(w, health, http.StatusOK)
}

// CreateCartRequest represents a cart creation request
type CreateCartRequest struct {
	CustomerID string `json:"customer_id"`
}

// CreateCartHandler creates a new cart
func CreateCartHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateCartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		correlationID := common.GetCorrelationID(r.Context())
		common.WriteErrorResponse(w, common.ErrBadRequest, http.StatusBadRequest, correlationID)
		return
	}

	if req.CustomerID == "" {
		correlationID := common.GetCorrelationID(r.Context())
		err := common.NewAppError("MISSING_CUSTOMER_ID", "Customer ID is required")
		common.WriteErrorResponse(w, err, http.StatusBadRequest, correlationID)
		return
	}

	cart, err := CreateCart(req.CustomerID)
	if err != nil {
		correlationID := common.GetCorrelationID(r.Context())
		appErr := common.NewAppError("CART_CREATION_FAILED", err.Error())
		common.WriteErrorResponse(w, appErr, http.StatusInternalServerError, correlationID)
		return
	}

	common.WriteJSONResponse(w, cart, http.StatusCreated)
}

// GetCartHandler retrieves a cart
func GetCartHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cartID := vars["id"]

	cart, err := GetCart(cartID)
	if err != nil {
		correlationID := common.GetCorrelationID(r.Context())
		common.WriteErrorResponse(w, common.ErrNotFound, http.StatusNotFound, correlationID)
		return
	}

	common.WriteJSONResponse(w, cart, http.StatusOK)
}

// AddItemRequest represents an add item request
type AddItemRequest struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

// AddItemHandler adds an item to the cart
func AddItemHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cartID := vars["id"]

	var req AddItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		correlationID := common.GetCorrelationID(r.Context())
		common.WriteErrorResponse(w, common.ErrBadRequest, http.StatusBadRequest, correlationID)
		return
	}

	if req.ProductID == "" || req.Quantity <= 0 {
		correlationID := common.GetCorrelationID(r.Context())
		err := common.NewAppError("INVALID_REQUEST", "Product ID and positive quantity are required")
		common.WriteErrorResponse(w, err, http.StatusBadRequest, correlationID)
		return
	}

	if err := AddItemToCart(cartID, req.ProductID, req.Quantity); err != nil {
		correlationID := common.GetCorrelationID(r.Context())

		// Determine appropriate error response
		statusCode := http.StatusInternalServerError
		appErr := common.NewAppError("ADD_ITEM_FAILED", err.Error())

		// If product not found, return 404
		if err.Error() == "product not found" {
			statusCode = http.StatusNotFound
			appErr = common.ErrNotFound
		}

		common.WriteErrorResponse(w, appErr, statusCode, correlationID)
		return
	}

	// Return updated cart
	cart, _ := GetCart(cartID)
	common.WriteJSONResponse(w, cart, http.StatusOK)
}

// RemoveItemHandler removes an item from the cart
func RemoveItemHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cartID := vars["id"]
	productID := vars["productId"]

	if err := RemoveItemFromCart(cartID, productID); err != nil {
		correlationID := common.GetCorrelationID(r.Context())

		statusCode := http.StatusInternalServerError
		appErr := common.NewAppError("REMOVE_ITEM_FAILED", err.Error())

		if err.Error() == "cart not found" || err.Error() == "item not found in cart" {
			statusCode = http.StatusNotFound
			appErr = common.ErrNotFound
		}

		common.WriteErrorResponse(w, appErr, statusCode, correlationID)
		return
	}

	// Return updated cart
	cart, _ := GetCart(cartID)
	common.WriteJSONResponse(w, cart, http.StatusOK)
}

// CheckoutResponse represents a checkout response
type CheckoutResponse struct {
	CartID     string  `json:"cart_id"`
	CustomerID string  `json:"customer_id"`
	Total      float64 `json:"total"`
	ItemCount  int     `json:"item_count"`
	Message    string  `json:"message"`
}

// CheckoutHandler initiates checkout for a cart
func CheckoutHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cartID := vars["id"]

	cart, err := GetCart(cartID)
	if err != nil {
		correlationID := common.GetCorrelationID(r.Context())
		common.WriteErrorResponse(w, common.ErrNotFound, http.StatusNotFound, correlationID)
		return
	}

	if len(cart.Items) == 0 {
		correlationID := common.GetCorrelationID(r.Context())
		err := common.NewAppError("EMPTY_CART", "Cannot checkout an empty cart")
		common.WriteErrorResponse(w, err, http.StatusBadRequest, correlationID)
		return
	}

	// In a real implementation, this would trigger the checkout process
	// For now, just return cart details
	response := CheckoutResponse{
		CartID:     cart.ID,
		CustomerID: cart.CustomerID,
		Total:      cart.Total,
		ItemCount:  len(cart.Items),
		Message:    "Ready for checkout",
	}

	common.WriteJSONResponse(w, response, http.StatusOK)
}