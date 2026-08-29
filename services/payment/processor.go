// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package payment

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	mrand "math/rand"
	"os"
	"time"

	"github.com/cardinalhq/griffin-commerce-demo/common"
	"github.com/cardinalhq/griffin-commerce-demo/common/faults"
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
	defer func() {
		if err := file.Close(); err != nil {
			slog.Warn("Failed to close config file", "error", err)
		}
	}()

	var config ProcessorsConfig
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return fmt.Errorf("failed to decode config: %w", err)
	}

	processors = config.Processors
	return nil
}

// ProcessPayment processes a payment with a specific processor.
// Fault-injection: payment.fail overrides the processor's effective failure
// rate when the knob's Target matches the chosen processor (or is empty,
// in which case the override applies to whichever processor was picked).
// Always emits griffin.payment.charges_total{processor, status} and the
// duration histogram.
func ProcessPayment(ctx context.Context, orderID string, amount float64, processorName string) (*common.Transaction, error) {
	start := time.Now()

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

	// Compute the effective failure rate. payment.fail with matching Target
	// (or empty Target) overrides the YAML default. We don't mutate the
	// processors map — the override is recomputed each call so clearing the
	// knob restores the YAML rate without explicit reset logic.
	failureRate := processor.FailureRate
	if k := faultsClient.Active(); k != nil && k.Key == "payment.fail" {
		if k.Target == "" || k.Target == processorName {
			failureRate = k.Probability
			faults.Record(ctx, k, 0)
		}
	}

	if shouldFail(failureRate) {
		transaction.Status = "failed"
		transaction.Message = getFailureMessage()
		slog.ErrorContext(ctx, "payment processor rejected charge",
			"processor", processorName,
			"processor_name", processor.Name,
			"order_id", orderID,
			"amount", amount,
			"reason", transaction.Message,
			"effective_failure_rate", failureRate,
			"baseline_failure_rate", processor.FailureRate,
		)
	} else {
		transaction.Status = "success"
		transaction.Message = "Payment processed successfully"
		slog.InfoContext(ctx, "payment processed",
			"processor", processorName,
			"processor_name", processor.Name,
			"order_id", orderID,
			"amount", amount,
			"transaction_id", transaction.ID,
			"status", "success",
		)
	}

	// Store transaction
	if err := transactionDB.Set(transaction.ID, transaction); err != nil {
		return nil, fmt.Errorf("failed to store transaction: %w", err)
	}

	durationMs := float64(time.Since(start).Milliseconds())
	faults.RecordPaymentCharge(ctx, processorName, transaction.Status, durationMs)

	return transaction, nil
}

// ReversePayment reverses a previously successful charge. This is the
// compensating action for a checkout that fails *after* the card was
// authorized — without it the customer is charged for an order that never
// ships, and every span in the trace still reports accurately.
//
// Reversal is idempotent: reversing an already-reversed transaction
// succeeds without recording a second reversal, so a client retry cannot
// manufacture a double refund.
func ReversePayment(ctx context.Context, orderID, transactionID string) (*common.Transaction, string, error) {
	transaction, err := GetTransaction(transactionID)
	if err != nil {
		return nil, "failed", err
	}

	// Guard against reversing someone else's charge if a caller crosses
	// wires: the order and the transaction must agree.
	if transaction.OrderID != orderID {
		return nil, "failed", fmt.Errorf("transaction %s belongs to order %s, not %s",
			transactionID, transaction.OrderID, orderID)
	}

	processorKey := processorKeyFor(transaction.Processor)

	if transaction.Status == "reversed" {
		faults.RecordPaymentReversal(ctx, processorKey, "already_reversed")
		return transaction, "already_reversed", nil
	}
	if transaction.Status != "success" {
		faults.RecordPaymentReversal(ctx, processorKey, "failed")
		return nil, "failed", fmt.Errorf("cannot reverse transaction %s in status %q",
			transactionID, transaction.Status)
	}

	transaction.Status = "reversed"
	transaction.Message = "Charge reversed"
	if err := transactionDB.Set(transaction.ID, transaction); err != nil {
		faults.RecordPaymentReversal(ctx, processorKey, "failed")
		return nil, "failed", fmt.Errorf("failed to store reversal: %w", err)
	}

	slog.InfoContext(ctx, "payment reversed",
		"processor", processorKey,
		"processor_name", transaction.Processor,
		"order_id", orderID,
		"transaction_id", transaction.ID,
		"amount", transaction.Amount,
		"status", "reversed",
	)
	faults.RecordPaymentReversal(ctx, processorKey, "reversed")

	return transaction, "reversed", nil
}

// processorKeyFor maps a processor display name ("KittyCard") back to its
// config key ("kittycard") so reversal metrics carry the same processor
// label as charge metrics. Falls back to the name when no key matches.
func processorKeyFor(name string) string {
	for key, p := range processors {
		if p.Name == name {
			return key
		}
	}
	return name
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
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID
		slog.Error("Failed to generate random transaction ID", "error", err)
		return fmt.Sprintf("TXN-%d", time.Now().UnixNano())
	}
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
