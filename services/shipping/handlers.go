// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package shipping

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/cardinalhq/griffin-commerce-demo/common"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/health", HealthHandler).Methods("GET")
	r.HandleFunc("/api/shipping/rates", GetRatesHandler).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/shipping/ship", CreateShipmentHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/shipping/{id}", GetShipmentHandler).Methods("GET", "OPTIONS")
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	health := common.HealthResponse{
		Status:    "healthy",
		Service:   "shipping-service",
		Version:   "1.0.0",
		Timestamp: time.Now(),
	}
	if err := common.WriteJSONResponse(w, health, http.StatusOK); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write response", "error", err)
	}
}

type RatesResponse struct {
	Carriers []CarrierRate `json:"carriers"`
}

type CarrierRate struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	Rate float64 `json:"rate"`
}

func GetRatesHandler(w http.ResponseWriter, r *http.Request) {
	response := RatesResponse{
		Carriers: []CarrierRate{},
	}

	carriersMutex.RLock()
	for id, carrier := range carriers {
		response.Carriers = append(response.Carriers, CarrierRate{
			ID:   id,
			Name: carrier.Name,
			Rate: carrier.Rate,
		})
	}
	carriersMutex.RUnlock()

	trace.SpanFromContext(r.Context()).SetAttributes(
		attribute.Int("shipping.carrier_count", len(response.Carriers)),
	)
	slog.InfoContext(r.Context(), "shipping rates queried", "carrier_count", len(response.Carriers))

	if err := common.WriteJSONResponse(w, response, http.StatusOK); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write response", "error", err)
	}
}

type ShipRequest struct {
	OrderID string `json:"order_id"`
	Carrier string `json:"carrier,omitempty"`
}

type ShipResponse struct {
	ShipmentID   string    `json:"shipment_id"`
	OrderID      string    `json:"order_id"`
	Carrier      string    `json:"carrier"`
	CarrierName  string    `json:"carrier_name"`
	TrackingNum  string    `json:"tracking_number,omitempty"`
	Status       string    `json:"status"`
	Cost         float64   `json:"cost"`
	EstDelivery  time.Time `json:"estimated_delivery,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

func CreateShipmentHandler(w http.ResponseWriter, r *http.Request) {
	var req ShipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		correlationID := common.GetCorrelationID(r.Context())
		common.WriteErrorResponse(r.Context(), w, common.ErrBadRequest, http.StatusBadRequest, correlationID)
		return
	}

	if req.OrderID == "" {
		correlationID := common.GetCorrelationID(r.Context())
		err := common.NewAppError("INVALID_REQUEST", "Order ID is required")
		common.WriteErrorResponse(r.Context(), w, err, http.StatusBadRequest, correlationID)
		return
	}

	span := trace.SpanFromContext(r.Context())
	span.SetAttributes(
		attribute.String("shipping.order_id", req.OrderID),
		attribute.String("shipping.requested_carrier", req.Carrier),
	)
	slog.InfoContext(r.Context(), "shipment requested",
		"order_id", req.OrderID, "requested_carrier", req.Carrier,
	)

	shipment, err := CreateShipment(r.Context(), req.OrderID, req.Carrier)

	response := ShipResponse{
		ShipmentID:  shipment.ID,
		OrderID:     shipment.OrderID,
		Carrier:     shipment.Carrier,
		CarrierName: shipment.CarrierName,
		Status:      shipment.Status,
		Cost:        shipment.Cost,
		CreatedAt:   shipment.CreatedAt,
	}

	if shipment.Status == "shipped" {
		response.TrackingNum = shipment.TrackingNum
		response.EstDelivery = shipment.EstDelivery
	}

	span.SetAttributes(
		attribute.String("shipping.shipment_id", shipment.ID),
		attribute.String("shipping.carrier", shipment.Carrier),
		attribute.String("shipping.status", shipment.Status),
		attribute.Float64("shipping.cost", shipment.Cost),
	)

	if err != nil {
		response.ErrorMessage = err.Error()
		if writeErr := common.WriteJSONResponse(w, response, http.StatusServiceUnavailable); writeErr != nil {
			slog.ErrorContext(r.Context(), "Failed to write response", "error", writeErr)
		}
		return
	}

	if err := common.WriteJSONResponse(w, response, http.StatusOK); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write response", "error", err)
	}
}

func GetShipmentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	shipmentID := vars["id"]

	trace.SpanFromContext(r.Context()).SetAttributes(
		attribute.String("shipping.shipment_id", shipmentID),
	)

	shipment, err := GetShipment(shipmentID)
	if err != nil {
		correlationID := common.GetCorrelationID(r.Context())
		common.WriteErrorResponse(r.Context(), w, common.ErrNotFound, http.StatusNotFound, correlationID)
		return
	}

	response := ShipResponse{
		ShipmentID:  shipment.ID,
		OrderID:     shipment.OrderID,
		Carrier:     shipment.Carrier,
		CarrierName: shipment.CarrierName,
		TrackingNum: shipment.TrackingNum,
		Status:      shipment.Status,
		Cost:        shipment.Cost,
		EstDelivery: shipment.EstDelivery,
		CreatedAt:   shipment.CreatedAt,
	}

	slog.InfoContext(r.Context(), "shipment status queried",
		"shipment_id", shipment.ID,
		"order_id", shipment.OrderID,
		"carrier", shipment.Carrier,
		"status", shipment.Status,
	)

	if err := common.WriteJSONResponse(w, response, http.StatusOK); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write response", "error", err)
	}
}
