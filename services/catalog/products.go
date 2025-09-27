package main

import (
	"fmt"
	"os"
	"sync"

	"github.com/griffincommerce/demo/common"
	"gopkg.in/yaml.v3"
)

var (
	productDB *common.MockDB
	mu        sync.RWMutex
)

func init() {
	productDB = common.NewMockDB()
}

// ProductsConfig represents the products configuration file
type ProductsConfig struct {
	Products []common.Product `yaml:"products"`
}

// LoadProducts loads products from a YAML file
func LoadProducts(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open products file: %w", err)
	}
	defer file.Close()

	var config ProductsConfig
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return fmt.Errorf("failed to decode products: %w", err)
	}

	// Store products in database
	for _, product := range config.Products {
		if err := productDB.Set(product.ID, product); err != nil {
			return fmt.Errorf("failed to store product %s: %w", product.ID, err)
		}
	}

	return nil
}

// GetAllProducts returns all products
func GetAllProducts() []common.Product {
	allData := productDB.GetAll()
	products := make([]common.Product, 0, len(allData))

	for _, v := range allData {
		if product, ok := v.(common.Product); ok {
			products = append(products, product)
		}
	}

	return products
}

// GetProduct returns a single product by ID
func GetProduct(id string) (*common.Product, error) {
	data, err := productDB.Get(id)
	if err != nil {
		return nil, fmt.Errorf("product not found: %s", id)
	}

	product, ok := data.(common.Product)
	if !ok {
		return nil, fmt.Errorf("invalid product data for ID: %s", id)
	}

	return &product, nil
}

// ReserveStock decreases the stock for a product
func ReserveStock(productID string, quantity int) error {
	mu.Lock()
	defer mu.Unlock()

	product, err := GetProduct(productID)
	if err != nil {
		return err
	}

	if product.Stock < quantity {
		return fmt.Errorf("insufficient stock: available=%d, requested=%d", product.Stock, quantity)
	}

	product.Stock -= quantity
	return productDB.Set(productID, *product)
}

// ReleaseStock increases the stock for a product
func ReleaseStock(productID string, quantity int) error {
	mu.Lock()
	defer mu.Unlock()

	product, err := GetProduct(productID)
	if err != nil {
		return err
	}

	product.Stock += quantity
	return productDB.Set(productID, *product)
}

// GetProductCount returns the total number of products
func GetProductCount() int {
	return productDB.Count()
}