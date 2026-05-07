// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package catalog

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/cardinalhq/griffin-commerce-demo/common"
	"github.com/cardinalhq/griffin-commerce-demo/common/faults"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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
		slog.ErrorContext(r.Context(), "Failed to write health response", "error", err)
	}
}

// GetProductsHandler returns all products.
func GetProductsHandler(w http.ResponseWriter, r *http.Request) {
	products := GetAllProducts()
	trace.SpanFromContext(r.Context()).SetAttributes(
		attribute.Int("catalog.product_count", len(products)),
	)
	slog.InfoContext(r.Context(), "products listed", "count", len(products))
	if err := common.WriteJSONResponse(w, products, http.StatusOK); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write products response", "error", err)
	}
}

// GetProductHandler returns a single product.
// Fault-injection hooks: catalog.error (for the targeted product id) returns
// a fault-injected response with the cause logged with trace_id so the
// side-drawer logs UX surfaces the explanation.
// Always emits griffin.catalog.product.{requests_total,duration_ms} so
// detect_outliers can locate per-product cohorts.
func GetProductHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	vars := mux.Vars(r)
	id := vars["id"]

	span := trace.SpanFromContext(r.Context())
	span.SetAttributes(attribute.String("product.id", id))

	statusCode := http.StatusOK
	defer func() {
		faults.RecordCatalogProduct(r.Context(), id, statusCode, float64(time.Since(start).Milliseconds()))
	}()

	if k, fired := faults.Probe(faultsClient, "catalog.error"); fired && k.Target == id {
		statusCode = k.StatusCode
		if statusCode == 0 {
			statusCode = http.StatusInternalServerError
		}
		slog.ErrorContext(r.Context(), "catalog: returning fault-injected error",
			"griffin.fault", k.Key,
			"product_id", id,
			"status_code", statusCode,
			"cause", "fault-injected: control plane catalog.error knob targeted this product",
		)
		faults.Record(r.Context(), k, 0)
		correlationID := common.GetCorrelationID(r.Context())
		common.WriteErrorResponse(r.Context(), w,
			common.NewAppError("CATALOG_UNAVAILABLE", "Product catalog temporarily unavailable"),
			statusCode, correlationID)
		return
	}

	product, err := GetProduct(id)
	if err != nil {
		statusCode = http.StatusNotFound
		correlationID := common.GetCorrelationID(r.Context())
		common.WriteErrorResponse(r.Context(), w, common.ErrNotFound, statusCode, correlationID)
		return
	}

	span.SetAttributes(
		attribute.String("product.name", product.Name),
		attribute.Float64("product.price", product.Price),
		attribute.Int("product.stock", product.Stock),
	)
	slog.InfoContext(r.Context(), "product viewed",
		"product_id", id,
		"product_name", product.Name,
		"price", product.Price,
		"stock", product.Stock,
	)

	if err := common.WriteJSONResponse(w, product, http.StatusOK); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write product response", "error", err)
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
		common.WriteErrorResponse(r.Context(), w, common.ErrBadRequest, http.StatusBadRequest, correlationID)
		return
	}

	if req.Quantity <= 0 {
		correlationID := common.GetCorrelationID(r.Context())
		badRequest := common.NewAppError("INVALID_QUANTITY", "Quantity must be greater than 0")
		common.WriteErrorResponse(r.Context(), w, badRequest, http.StatusBadRequest, correlationID)
		return
	}

	if err := ReserveStock(id, req.Quantity); err != nil {
		correlationID := common.GetCorrelationID(r.Context())
		appErr := common.NewAppError("RESERVATION_FAILED", err.Error())
		common.WriteErrorResponse(r.Context(), w, appErr, http.StatusConflict, correlationID)
		return
	}

	product, _ := GetProduct(id)
	response := StockResponse{
		ProductID: id,
		Stock:     product.Stock,
		Message:   "Stock reserved successfully",
	}

	trace.SpanFromContext(r.Context()).SetAttributes(
		attribute.String("product.id", id),
		attribute.Int("stock.reserved_quantity", req.Quantity),
		attribute.Int("stock.remaining", product.Stock),
	)
	slog.InfoContext(r.Context(), "stock reserved",
		"product_id", id, "quantity", req.Quantity, "remaining_stock", product.Stock,
	)

	if err := common.WriteJSONResponse(w, response, http.StatusOK); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write response", "error", err)
	}
}

// ReleaseStockHandler releases reserved stock
func ReleaseStockHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var req StockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		correlationID := common.GetCorrelationID(r.Context())
		common.WriteErrorResponse(r.Context(), w, common.ErrBadRequest, http.StatusBadRequest, correlationID)
		return
	}

	if req.Quantity <= 0 {
		correlationID := common.GetCorrelationID(r.Context())
		badRequest := common.NewAppError("INVALID_QUANTITY", "Quantity must be greater than 0")
		common.WriteErrorResponse(r.Context(), w, badRequest, http.StatusBadRequest, correlationID)
		return
	}

	if err := ReleaseStock(id, req.Quantity); err != nil {
		correlationID := common.GetCorrelationID(r.Context())
		appErr := common.NewAppError("RELEASE_FAILED", err.Error())
		common.WriteErrorResponse(r.Context(), w, appErr, http.StatusNotFound, correlationID)
		return
	}

	product, _ := GetProduct(id)
	response := StockResponse{
		ProductID: id,
		Stock:     product.Stock,
		Message:   "Stock released successfully",
	}

	trace.SpanFromContext(r.Context()).SetAttributes(
		attribute.String("product.id", id),
		attribute.Int("stock.released_quantity", req.Quantity),
		attribute.Int("stock.remaining", product.Stock),
	)
	slog.InfoContext(r.Context(), "stock released",
		"product_id", id, "quantity", req.Quantity, "remaining_stock", product.Stock,
	)

	if err := common.WriteJSONResponse(w, response, http.StatusOK); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write response", "error", err)
	}
}
