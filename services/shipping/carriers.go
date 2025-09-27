package main

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type CarrierConfig struct {
	Name        string  `yaml:"name"`
	Rate        float64 `yaml:"rate"`
	FailureRate float64 `yaml:"failure_rate"`
}

type CarriersConfig struct {
	Carriers map[string]CarrierConfig `yaml:"carriers"`
}

var (
	carriers       map[string]CarrierConfig
	carriersMutex  sync.RWMutex
	shipments      map[string]*Shipment
	shipmentsMutex sync.RWMutex
)

type Shipment struct {
	ID          string    `json:"id"`
	OrderID     string    `json:"order_id"`
	Carrier     string    `json:"carrier"`
	CarrierName string    `json:"carrier_name"`
	TrackingNum string    `json:"tracking_number"`
	Status      string    `json:"status"`
	Cost        float64   `json:"cost"`
	EstDelivery time.Time `json:"estimated_delivery"`
	CreatedAt   time.Time `json:"created_at"`
}

func LoadCarrierConfig(filename string) error {
	data, err := yaml.Marshal(&CarriersConfig{
		Carriers: map[string]CarrierConfig{
			"ponyexpress": {
				Name:        "Pony Express Ground",
				Rate:        9.99,
				FailureRate: 0.05,
			},
			"avianair": {
				Name:        "Avian Air Express",
				Rate:        19.99,
				FailureRate: 0.10,
			},
			"catcarrier": {
				Name:        "Cat Carrier Delivery",
				Rate:        14.99,
				FailureRate: 0.25,
			},
		},
	})
	if err != nil {
		return err
	}

	var config CarriersConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return err
	}

	carriersMutex.Lock()
	carriers = config.Carriers
	carriersMutex.Unlock()

	return nil
}

func InitShipmentStorage() {
	shipmentsMutex.Lock()
	shipments = make(map[string]*Shipment)
	shipmentsMutex.Unlock()
}

func GetCarriers() []CarrierConfig {
	carriersMutex.RLock()
	defer carriersMutex.RUnlock()

	var result []CarrierConfig
	for _, carrier := range carriers {
		result = append(result, carrier)
	}
	return result
}

func CreateShipment(orderID string, carrierID string) (*Shipment, error) {
	carriersMutex.RLock()
	carrier, exists := carriers[carrierID]
	carriersMutex.RUnlock()

	if !exists {
		var availableCarriers []string
		carriersMutex.RLock()
		for id := range carriers {
			availableCarriers = append(availableCarriers, id)
		}
		carriersMutex.RUnlock()

		if len(availableCarriers) > 0 {
			randomIndex := rand.Intn(len(availableCarriers))
			carrierID = availableCarriers[randomIndex]
			carriersMutex.RLock()
			carrier = carriers[carrierID]
			carriersMutex.RUnlock()
			log.Printf("Selected random carrier: %s", carrierID)
		} else {
			return nil, fmt.Errorf("no carriers available")
		}
	}

	if rand.Float64() < carrier.FailureRate {
		log.Printf("Shipping failed with %s (%.0f%% failure rate)", carrierID, carrier.FailureRate*100)
		return &Shipment{
			ID:          generateShipmentID(),
			OrderID:     orderID,
			Carrier:     carrierID,
			CarrierName: carrier.Name,
			Status:      "failed",
			Cost:        carrier.Rate,
			CreatedAt:   time.Now(),
		}, fmt.Errorf("shipping failed: %s is experiencing issues", carrier.Name)
	}

	shipment := &Shipment{
		ID:          generateShipmentID(),
		OrderID:     orderID,
		Carrier:     carrierID,
		CarrierName: carrier.Name,
		TrackingNum: generateTrackingNumber(carrierID),
		Status:      "shipped",
		Cost:        carrier.Rate,
		EstDelivery: calculateEstimatedDelivery(carrierID),
		CreatedAt:   time.Now(),
	}

	shipmentsMutex.Lock()
	shipments[shipment.ID] = shipment
	shipmentsMutex.Unlock()

	log.Printf("Shipment created: %s via %s", shipment.ID, carrier.Name)
	return shipment, nil
}

func GetShipment(shipmentID string) (*Shipment, error) {
	shipmentsMutex.RLock()
	defer shipmentsMutex.RUnlock()

	shipment, exists := shipments[shipmentID]
	if !exists {
		return nil, fmt.Errorf("shipment not found: %s", shipmentID)
	}

	return shipment, nil
}

func generateShipmentID() string {
	return fmt.Sprintf("SHIP-%d-%d", time.Now().Unix(), rand.Intn(10000))
}

func generateTrackingNumber(carrierID string) string {
	prefix := ""
	switch carrierID {
	case "ponyexpress":
		prefix = "PE"
	case "avianair":
		prefix = "AA"
	case "catcarrier":
		prefix = "CC"
	default:
		prefix = "XX"
	}
	return fmt.Sprintf("%s%d%d", prefix, time.Now().Unix()%1000000, rand.Intn(10000))
}

func calculateEstimatedDelivery(carrierID string) time.Time {
	days := 3
	switch carrierID {
	case "ponyexpress":
		days = 5
	case "avianair":
		days = 2
	case "catcarrier":
		days = 3 + rand.Intn(4)
	}
	return time.Now().AddDate(0, 0, days)
}