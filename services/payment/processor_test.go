// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package payment

import (
	"context"
	"testing"
	"time"

	"github.com/cardinalhq/griffin-commerce-demo/common"
)

// seedTransaction installs a transaction directly so tests don't depend on
// ProcessPayment's random failure roll.
func seedTransaction(t *testing.T, id, orderID, status string) *common.Transaction {
	t.Helper()
	InitTransactionStorage()
	processors = map[string]ProcessorConfig{
		"kittycard": {Name: "KittyCard", FailureRate: 0.2, LatencyMs: 1},
	}
	txn := &common.Transaction{
		ID:        id,
		OrderID:   orderID,
		Amount:    147.23,
		Status:    status,
		Processor: "KittyCard",
		CreatedAt: time.Now(),
	}
	if err := transactionDB.Set(txn.ID, txn); err != nil {
		t.Fatalf("seed transaction: %v", err)
	}
	return txn
}

func TestReversePaymentReversesSuccessfulCharge(t *testing.T) {
	seedTransaction(t, "TXN-1", "ORDER-1", "success")

	txn, outcome, err := ReversePayment(context.Background(), "ORDER-1", "TXN-1")
	if err != nil {
		t.Fatalf("reverse: %v", err)
	}
	if outcome != "reversed" {
		t.Errorf("outcome = %q, want reversed", outcome)
	}
	if txn.Status != "reversed" {
		t.Errorf("status = %q, want reversed", txn.Status)
	}

	// The stored transaction must reflect the reversal, otherwise a later
	// read reports the charge as still authorized.
	stored, err := GetTransaction("TXN-1")
	if err != nil {
		t.Fatalf("get after reverse: %v", err)
	}
	if stored.Status != "reversed" {
		t.Errorf("stored status = %q, want reversed", stored.Status)
	}
}

func TestReversePaymentIsIdempotent(t *testing.T) {
	seedTransaction(t, "TXN-2", "ORDER-2", "success")

	if _, outcome, err := ReversePayment(context.Background(), "ORDER-2", "TXN-2"); err != nil || outcome != "reversed" {
		t.Fatalf("first reverse: outcome=%q err=%v", outcome, err)
	}

	// A retry must not error and must not present as a second reversal —
	// otherwise a client retry manufactures a double refund.
	_, outcome, err := ReversePayment(context.Background(), "ORDER-2", "TXN-2")
	if err != nil {
		t.Fatalf("second reverse: %v", err)
	}
	if outcome != "already_reversed" {
		t.Errorf("outcome = %q, want already_reversed", outcome)
	}
}

func TestReversePaymentRejectsOrderMismatch(t *testing.T) {
	seedTransaction(t, "TXN-3", "ORDER-3", "success")

	if _, _, err := ReversePayment(context.Background(), "ORDER-OTHER", "TXN-3"); err == nil {
		t.Fatal("expected error reversing a transaction belonging to another order")
	}

	// The charge must remain authorized after a rejected reversal.
	stored, err := GetTransaction("TXN-3")
	if err != nil {
		t.Fatalf("get after rejected reverse: %v", err)
	}
	if stored.Status != "success" {
		t.Errorf("status = %q, want success (unchanged)", stored.Status)
	}
}

func TestReversePaymentRejectsFailedCharge(t *testing.T) {
	seedTransaction(t, "TXN-4", "ORDER-4", "failed")

	if _, outcome, err := ReversePayment(context.Background(), "ORDER-4", "TXN-4"); err == nil {
		t.Fatalf("expected error reversing a failed charge, got outcome=%q", outcome)
	}
}

func TestReversePaymentUnknownTransaction(t *testing.T) {
	seedTransaction(t, "TXN-5", "ORDER-5", "success")

	if _, _, err := ReversePayment(context.Background(), "ORDER-5", "TXN-MISSING"); err == nil {
		t.Fatal("expected error reversing an unknown transaction")
	}
}

func TestProcessorKeyForMapsDisplayNameToConfigKey(t *testing.T) {
	seedTransaction(t, "TXN-6", "ORDER-6", "success")

	// Reversal metrics must carry the same processor label as charge
	// metrics, which use the config key rather than the display name.
	if got := processorKeyFor("KittyCard"); got != "kittycard" {
		t.Errorf("processorKeyFor(KittyCard) = %q, want kittycard", got)
	}
	if got := processorKeyFor("Unmapped"); got != "Unmapped" {
		t.Errorf("processorKeyFor(Unmapped) = %q, want passthrough", got)
	}
}
