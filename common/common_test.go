// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package common

import (
	"testing"
)

func TestMockDB(t *testing.T) {
	db := NewMockDB()

	// Test Set and Get
	err := db.Set("key1", "value1")
	if err != nil {
		t.Errorf("Failed to set key: %v", err)
	}

	val, err := db.Get("key1")
	if err != nil {
		t.Errorf("Failed to get key: %v", err)
	}

	if val != "value1" {
		t.Errorf("Expected value1, got %v", val)
	}

	// Test Exists
	if !db.Exists("key1") {
		t.Error("Key should exist")
	}

	// Test Delete
	err = db.Delete("key1")
	if err != nil {
		t.Errorf("Failed to delete key: %v", err)
	}

	if db.Exists("key1") {
		t.Error("Key should not exist after deletion")
	}
}

func TestAppError(t *testing.T) {
	err := NewAppError("TEST_ERROR", "This is a test error")

	if err.Code != "TEST_ERROR" {
		t.Errorf("Expected TEST_ERROR, got %s", err.Code)
	}

	if err.Error() != "This is a test error" {
		t.Errorf("Expected 'This is a test error', got %s", err.Error())
	}
}
