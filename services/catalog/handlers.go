// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package catalog

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
	r.HandleFunc("/api/products", GetProductsHandler).Methods("GET")
	r.HandleFunc("/api/products/{id}", GetProductHandler).Methods("GET")
	r.HandleFunc("/api/products/{id}/reserve", ReserveStockHandler).Methods("POST")
	r.HandleFunc("/api/products/{id}/release", ReleaseStockHandler).Methods("POST")
}

// HealthHandler returns service health status
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	health := common.HealthResponse{
		Status:    "healthy",
		Service:   "catalog-service",
		Version:   "1.0.0",
		Timestamp: time.Now(),
	}
	if err := common.WriteJSONResponse(w, health, http.StatusOK); err != nil {
		slog.Error("Failed to write health response", "error", err)
	}
}

// GetProductsHandler returns all products
func GetProductsHandler(w http.ResponseWriter, r *http.Request) {
	products := GetAllProducts()
	if err := common.WriteJSONResponse(w, products, http.StatusOK); err != nil {
		slog.Error("Failed to write products response", "error", err)
	}
}

// GetProductHandler returns a single product
func GetProductHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	product, err := GetProduct(id)
	if err != nil {
		correlationID := common.GetCorrelationID(r.Context())
		common.WriteErrorResponse(w, common.ErrNotFound, http.StatusNotFound, correlationID)
		return
	}

	if err := common.WriteJSONResponse(w, product, http.StatusOK); err != nil {
		slog.Error("Failed to write product response", "error", err)
	}
}

// StockRequest represents a stock reservation/release request
type StockRequest struct {
	Quantity int `json:"quantity"`
}

// StockResponse represents a stock operation response
type StockResponse struct {
	ProductID string `json:"product_id"`
	Stock     int    `json:"stock"`
	Message   string `json:"message"`
}

// ReserveStockHandler reserves stock for a product
func ReserveStockHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var req StockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		correlationID := common.GetCorrelationID(r.Context())
		common.WriteErrorResponse(w, common.ErrBadRequest, http.StatusBadRequest, correlationID)
		return
	}

	if req.Quantity <= 0 {
		correlationID := common.GetCorrelationID(r.Context())
		badRequest := common.NewAppError("INVALID_QUANTITY", "Quantity must be greater than 0")
		common.WriteErrorResponse(w, badRequest, http.StatusBadRequest, correlationID)
		return
	}

	if err := ReserveStock(id, req.Quantity); err != nil {
		correlationID := common.GetCorrelationID(r.Context())
		appErr := common.NewAppError("RESERVATION_FAILED", err.Error())
		common.WriteErrorResponse(w, appErr, http.StatusConflict, correlationID)
		return
	}

	product, _ := GetProduct(id)
	response := StockResponse{
		ProductID: id,
		Stock:     product.Stock,
		Message:   "Stock reserved successfully",
	}

	if err := common.WriteJSONResponse(w, response, http.StatusOK); err != nil {
		slog.Error("Failed to write response", "error", err)
	}
}

// ReleaseStockHandler releases reserved stock
func ReleaseStockHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var req StockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		correlationID := common.GetCorrelationID(r.Context())
		common.WriteErrorResponse(w, common.ErrBadRequest, http.StatusBadRequest, correlationID)
		return
	}

	if req.Quantity <= 0 {
		correlationID := common.GetCorrelationID(r.Context())
		badRequest := common.NewAppError("INVALID_QUANTITY", "Quantity must be greater than 0")
		common.WriteErrorResponse(w, badRequest, http.StatusBadRequest, correlationID)
		return
	}

	if err := ReleaseStock(id, req.Quantity); err != nil {
		correlationID := common.GetCorrelationID(r.Context())
		appErr := common.NewAppError("RELEASE_FAILED", err.Error())
		common.WriteErrorResponse(w, appErr, http.StatusNotFound, correlationID)
		return
	}

	product, _ := GetProduct(id)
	response := StockResponse{
		ProductID: id,
		Stock:     product.Stock,
		Message:   "Stock released successfully",
	}

	if err := common.WriteJSONResponse(w, response, http.StatusOK); err != nil {
		slog.Error("Failed to write response", "error", err)
	}
}
