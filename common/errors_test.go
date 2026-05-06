// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package common

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestNewAppError(t *testing.T) {
	err := NewAppError("TEST_ERROR", "This is a test error")

	if err.Code != "TEST_ERROR" {
		t.Errorf("Expected error code TEST_ERROR, got %s", err.Code)
	}

	if err.Message != "This is a test error" {
		t.Errorf("Expected error message 'This is a test error', got %s", err.Message)
	}
}

func TestWriteErrorResponse(t *testing.T) {
	rec := httptest.NewRecorder()
	err := NewAppError("TEST_ERROR", "Test error message")

	WriteErrorResponse(context.Background(), rec, err, 400, "test-correlation-id")

	if rec.Code != 400 {
		t.Errorf("Expected status code 400, got %d", rec.Code)
	}

	var response ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Error.Code != "TEST_ERROR" {
		t.Errorf("Expected error code TEST_ERROR, got %s", response.Error.Code)
	}

	if response.Error.Message != "Test error message" {
		t.Errorf("Expected error message 'Test error message', got %s", response.Error.Message)
	}

	if response.CorrelationID != "test-correlation-id" {
		t.Errorf("Expected correlation ID 'test-correlation-id', got %s", response.CorrelationID)
	}
}

func TestWriteJSONResponse(t *testing.T) {
	rec := httptest.NewRecorder()
	data := map[string]string{"test": "value"}

	if err := WriteJSONResponse(rec, data, 200); err != nil {
		t.Fatalf("Failed to write JSON response: %v", err)
	}

	if rec.Code != 200 {
		t.Errorf("Expected status code 200, got %d", rec.Code)
	}

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", rec.Header().Get("Content-Type"))
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["test"] != "value" {
		t.Errorf("Expected test=value, got test=%s", response["test"])
	}
}
