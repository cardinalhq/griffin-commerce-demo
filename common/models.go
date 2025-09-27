package common

import "time"

// Product represents a product in the catalog
type Product struct {
	ID       string  `json:"id" yaml:"id"`
	Name     string  `json:"name" yaml:"name"`
	Price    float64 `json:"price" yaml:"price"`
	Stock    int     `json:"stock" yaml:"stock"`
	Category string  `json:"category" yaml:"category"`
	ImageURL string  `json:"image_url" yaml:"image_url"`
}

// Cart represents a shopping cart
type Cart struct {
	ID         string     `json:"id"`
	CustomerID string     `json:"customer_id"`
	Items      []CartItem `json:"items"`
	Total      float64    `json:"total"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// CartItem represents an item in the cart
type CartItem struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
	Subtotal  float64 `json:"subtotal"`
}

// Transaction represents a payment transaction
type Transaction struct {
	ID        string    `json:"id"`
	OrderID   string    `json:"order_id"`
	Amount    float64   `json:"amount"`
	Status    string    `json:"status"` // "success" or "failed"
	Processor string    `json:"processor"`
	CreatedAt time.Time `json:"created_at"`
	Message   string    `json:"message,omitempty"`
}

// Shipment represents a shipping record
type Shipment struct {
	ID        string    `json:"id"`
	OrderID   string    `json:"order_id"`
	Carrier   string    `json:"carrier"`
	Status    string    `json:"status"` // "submitted" or "failed"
	Cost      float64   `json:"cost"`
	CreatedAt time.Time `json:"created_at"`
	Message   string    `json:"message,omitempty"`
}

// ShippingRate represents a shipping rate quote
type ShippingRate struct {
	Carrier string  `json:"carrier"`
	Name    string  `json:"name"`
	Cost    float64 `json:"cost"`
}

// Recommendation represents a product recommendation
type Recommendation struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

// HealthResponse represents a health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Service   string    `json:"service"`
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
}
