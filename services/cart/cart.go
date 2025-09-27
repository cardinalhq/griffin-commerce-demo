package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/griffincommerce/demo/common"
)

var (
	cartDB *common.MockDB
	mu     sync.RWMutex
)

// InitCartStorage initializes the cart storage
func InitCartStorage() {
	cartDB = common.NewMockDB()
}

// CreateCart creates a new cart for a customer
func CreateCart(customerID string) (*common.Cart, error) {
	if customerID == "" {
		return nil, fmt.Errorf("customer ID is required")
	}

	cart := &common.Cart{
		ID:         generateCartID(),
		CustomerID: customerID,
		Items:      []common.CartItem{},
		Total:      0,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := cartDB.Set(cart.ID, cart); err != nil {
		return nil, err
	}

	return cart, nil
}

// GetCart retrieves a cart by ID
func GetCart(cartID string) (*common.Cart, error) {
	data, err := cartDB.Get(cartID)
	if err != nil {
		return nil, fmt.Errorf("cart not found: %s", cartID)
	}

	cart, ok := data.(*common.Cart)
	if !ok {
		return nil, fmt.Errorf("invalid cart data")
	}

	return cart, nil
}

// AddItemToCart adds an item to the cart
func AddItemToCart(cartID string, productID string, quantity int) error {
	mu.Lock()
	defer mu.Unlock()

	// Get cart
	cart, err := GetCart(cartID)
	if err != nil {
		return err
	}

	// Get product details from catalog service
	product, err := GetProduct(productID)
	if err != nil {
		return fmt.Errorf("failed to get product: %w", err)
	}

	// Check if item already exists in cart
	for i, item := range cart.Items {
		if item.ProductID == productID {
			// Update quantity
			cart.Items[i].Quantity += quantity
			cart.Items[i].Subtotal = cart.Items[i].Price * float64(cart.Items[i].Quantity)
			cart.UpdatedAt = time.Now()
			return updateCartTotalAndSave(cart)
		}
	}

	// Add new item
	newItem := common.CartItem{
		ProductID: productID,
		Name:      product.Name,
		Price:     product.Price,
		Quantity:  quantity,
		Subtotal:  product.Price * float64(quantity),
	}

	cart.Items = append(cart.Items, newItem)
	cart.UpdatedAt = time.Now()

	return updateCartTotalAndSave(cart)
}

// RemoveItemFromCart removes an item from the cart
func RemoveItemFromCart(cartID string, productID string) error {
	mu.Lock()
	defer mu.Unlock()

	cart, err := GetCart(cartID)
	if err != nil {
		return err
	}

	// Find and remove item
	found := false
	newItems := []common.CartItem{}
	for _, item := range cart.Items {
		if item.ProductID != productID {
			newItems = append(newItems, item)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("item not found in cart")
	}

	cart.Items = newItems
	cart.UpdatedAt = time.Now()

	return updateCartTotalAndSave(cart)
}

// updateCartTotalAndSave recalculates the cart total and saves it
func updateCartTotalAndSave(cart *common.Cart) error {
	total := 0.0
	for _, item := range cart.Items {
		total += item.Subtotal
	}
	cart.Total = total

	return cartDB.Set(cart.ID, cart)
}

// generateCartID generates a unique cart ID
func generateCartID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "CART-" + hex.EncodeToString(bytes)
}
