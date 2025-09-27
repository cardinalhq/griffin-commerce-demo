package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	mrand "math/rand"
	"os"
	"time"

	"github.com/griffincommerce/demo/common"
	"gopkg.in/yaml.v3"
)

var (
	transactionDB *common.MockDB
	processors    map[string]ProcessorConfig
	random        *mrand.Rand
)

// ProcessorConfig represents a payment processor configuration
type ProcessorConfig struct {
	Name        string  `yaml:"name"`
	FailureRate float64 `yaml:"failure_rate"`
	LatencyMs   int     `yaml:"latency_ms"`
}

// ProcessorsConfig represents the processors configuration file
type ProcessorsConfig struct {
	Processors map[string]ProcessorConfig `yaml:"processors"`
}

// InitTransactionStorage initializes the transaction storage
func InitTransactionStorage() {
	transactionDB = common.NewMockDB()
	random = mrand.New(mrand.NewSource(time.Now().UnixNano()))
}

// LoadProcessorConfig loads processor configuration from a YAML file
func LoadProcessorConfig(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	var config ProcessorsConfig
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return fmt.Errorf("failed to decode config: %w", err)
	}

	processors = config.Processors
	return nil
}

// ProcessPayment processes a payment with a specific processor
func ProcessPayment(orderID string, amount float64, processorName string) (*common.Transaction, error) {
	// Select processor
	if processorName == "" {
		processorName = selectRandomProcessor()
	}

	processor, exists := processors[processorName]
	if !exists {
		return nil, fmt.Errorf("unknown processor: %s", processorName)
	}

	// Simulate processing latency
	time.Sleep(time.Duration(processor.LatencyMs) * time.Millisecond)

	// Create transaction
	transaction := &common.Transaction{
		ID:        generateTransactionID(),
		OrderID:   orderID,
		Amount:    amount,
		Processor: processor.Name,
		CreatedAt: time.Now(),
	}

	// Determine if payment should fail based on failure rate
	if shouldFail(processor.FailureRate) {
		transaction.Status = "failed"
		transaction.Message = getFailureMessage()
	} else {
		transaction.Status = "success"
		transaction.Message = "Payment processed successfully"
	}

	// Store transaction
	if err := transactionDB.Set(transaction.ID, transaction); err != nil {
		return nil, fmt.Errorf("failed to store transaction: %w", err)
	}

	return transaction, nil
}

// GetTransaction retrieves a transaction by ID
func GetTransaction(transactionID string) (*common.Transaction, error) {
	data, err := transactionDB.Get(transactionID)
	if err != nil {
		return nil, fmt.Errorf("transaction not found: %s", transactionID)
	}

	transaction, ok := data.(*common.Transaction)
	if !ok {
		return nil, fmt.Errorf("invalid transaction data")
	}

	return transaction, nil
}

// selectRandomProcessor randomly selects a processor
func selectRandomProcessor() string {
	names := []string{"puppypay", "kittycard", "doggiecoin"}
	return names[random.Intn(len(names))]
}

// shouldFail determines if a payment should fail based on the failure rate
func shouldFail(failureRate float64) bool {
	return random.Float64() < failureRate
}

// generateTransactionID generates a unique transaction ID
func generateTransactionID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "TXN-" + hex.EncodeToString(bytes)
}

// getFailureMessage returns a random failure message
func getFailureMessage() string {
	messages := []string{
		"Insufficient funds",
		"Card declined",
		"Processing error",
		"Invalid payment method",
		"Transaction timeout",
		"Processor unavailable",
	}
	return messages[random.Intn(len(messages))]
}

// roundToTwoDecimals rounds a float to 2 decimal places
func roundToTwoDecimals(val float64) float64 {
	return math.Round(val*100) / 100
}