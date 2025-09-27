package common

import (
	"fmt"
	"sync"
)

// MockDB is a simple in-memory database using a map with mutex
type MockDB struct {
	mu   sync.RWMutex
	data map[string]interface{}
}

// NewMockDB creates a new mock database
func NewMockDB() *MockDB {
	return &MockDB{
		data: make(map[string]interface{}),
	}
}

// Set stores a value with the given key
func (db *MockDB) Set(key string, value interface{}) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	db.data[key] = value
	return nil
}

// Get retrieves a value by key
func (db *MockDB) Get(key string) (interface{}, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	value, exists := db.data[key]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	return value, nil
}

// Delete removes a value by key
func (db *MockDB) Delete(key string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if _, exists := db.data[key]; !exists {
		return fmt.Errorf("key not found: %s", key)
	}

	delete(db.data, key)
	return nil
}

// Exists checks if a key exists
func (db *MockDB) Exists(key string) bool {
	db.mu.RLock()
	defer db.mu.RUnlock()

	_, exists := db.data[key]
	return exists
}

// GetAll returns all keys and values
func (db *MockDB) GetAll() map[string]interface{} {
	db.mu.RLock()
	defer db.mu.RUnlock()

	// Create a copy to avoid race conditions
	result := make(map[string]interface{}, len(db.data))
	for k, v := range db.data {
		result[k] = v
	}

	return result
}

// Clear removes all data from the database
func (db *MockDB) Clear() {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.data = make(map[string]interface{})
}

// Count returns the number of items in the database
func (db *MockDB) Count() int {
	db.mu.RLock()
	defer db.mu.RUnlock()

	return len(db.data)
}
