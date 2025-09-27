package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/griffincommerce/demo/common"
)

func RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/health", HealthHandler).Methods("GET")
	r.HandleFunc("/api/shipping/rates", GetRatesHandler).Methods("GET")
	r.HandleFunc("/api/shipping/ship", CreateShipmentHandler).Methods("POST")
	r.HandleFunc("/api/shipping/{id}", GetShipmentHandler).Methods("GET")
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	health := common.HealthResponse{
		Status:    "healthy",
		Service:   "shipping-service",
		Version:   "1.0.0",
		Timestamp: time.Now(),
	}
	common.WriteJSONResponse(w, health, http.StatusOK)
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

	common.WriteJSONResponse(w, response, http.StatusOK)
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
		common.WriteErrorResponse(w, common.ErrBadRequest, http.StatusBadRequest, correlationID)
		return
	}

	if req.OrderID == "" {
		correlationID := common.GetCorrelationID(r.Context())
		err := common.NewAppError("INVALID_REQUEST", "Order ID is required")
		common.WriteErrorResponse(w, err, http.StatusBadRequest, correlationID)
		return
	}

	shipment, err := CreateShipment(req.OrderID, req.Carrier)

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

	if err != nil {
		response.ErrorMessage = err.Error()
		common.WriteJSONResponse(w, response, http.StatusServiceUnavailable)
		return
	}

	common.WriteJSONResponse(w, response, http.StatusOK)
}

func GetShipmentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	shipmentID := vars["id"]

	shipment, err := GetShipment(shipmentID)
	if err != nil {
		correlationID := common.GetCorrelationID(r.Context())
		common.WriteErrorResponse(w, common.ErrNotFound, http.StatusNotFound, correlationID)
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

	common.WriteJSONResponse(w, response, http.StatusOK)
}
