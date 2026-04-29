// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package cart

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
	r.HandleFunc("/api/cart/create", CreateCartHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/cart/{id}", GetCartHandler).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/cart/{id}/add", AddItemHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/cart/{id}/item/{productId}", RemoveItemHandler).Methods("DELETE", "OPTIONS")
	r.HandleFunc("/api/cart/{id}/clear", ClearCartHandler).Methods("DELETE", "OPTIONS")
	r.HandleFunc("/api/cart/{id}/checkout", CheckoutHandler).Methods("POST", "OPTIONS")
}

// HealthHandler returns service health status
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	health := common.HealthResponse{
		Status:    "healthy",
		Service:   "cart-service",
		Version:   "1.0.0",
		Timestamp: time.Now(),
	}
	if err := common.WriteJSONResponse(w, health, http.StatusOK); err != nil {
		slog.Error("Failed to write health response", "error", err)
	}
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

	if err := common.WriteJSONResponse(w, cart, http.StatusCreated); err != nil {
		slog.Error("Failed to write cart response", "error", err)
	}
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

	if err := common.WriteJSONResponse(w, cart, http.StatusOK); err != nil {
		slog.Error("Failed to write cart response", "error", err)
	}
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
	if err := common.WriteJSONResponse(w, cart, http.StatusOK); err != nil {
		slog.Error("Failed to write cart response", "error", err)
	}
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
	if err := common.WriteJSONResponse(w, cart, http.StatusOK); err != nil {
		slog.Error("Failed to write cart response", "error", err)
	}
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

	if err := common.WriteJSONResponse(w, response, http.StatusOK); err != nil {
		slog.Error("Failed to write checkout response", "error", err)
	}
}

// ClearCartHandler clears all items from a cart
func ClearCartHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cartID := vars["id"]

	cart, err := GetCart(cartID)
	if err != nil {
		correlationID := common.GetCorrelationID(r.Context())
		common.WriteErrorResponse(w, common.ErrNotFound, http.StatusNotFound, correlationID)
		return
	}

	// Clear all items
	cart.Items = []common.CartItem{}
	cart.Total = 0
	cart.UpdatedAt = time.Now()

	// Save the cleared cart
	if err := cartDB.Set(cart.ID, cart); err != nil {
		correlationID := common.GetCorrelationID(r.Context())
		appErr := common.NewAppError("CLEAR_FAILED", "Failed to clear cart")
		common.WriteErrorResponse(w, appErr, http.StatusInternalServerError, correlationID)
		return
	}

	if err := common.WriteJSONResponse(w, cart, http.StatusOK); err != nil {
		slog.Error("Failed to write cart response", "error", err)
	}
}
