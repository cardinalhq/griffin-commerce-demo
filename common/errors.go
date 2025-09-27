package common

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// AppError represents an application error
type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error implements the error interface
func (e AppError) Error() string {
	return e.Message
}

// Common error codes
var (
	ErrNotFound      = AppError{Code: "NOT_FOUND", Message: "Resource not found"}
	ErrBadRequest    = AppError{Code: "BAD_REQUEST", Message: "Invalid request"}
	ErrInternalError = AppError{Code: "INTERNAL_ERROR", Message: "Internal server error"}
	ErrUnauthorized  = AppError{Code: "UNAUTHORIZED", Message: "Unauthorized"}
	ErrForbidden     = AppError{Code: "FORBIDDEN", Message: "Forbidden"}
)

// ErrorResponse is the JSON error response structure
type ErrorResponse struct {
	Error         AppError `json:"error"`
	CorrelationID string   `json:"correlation_id,omitempty"`
}

// WriteErrorResponse writes an error response to the client
func WriteErrorResponse(w http.ResponseWriter, err AppError, statusCode int, correlationID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := ErrorResponse{
		Error:         err,
		CorrelationID: correlationID,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("Failed to encode error response", "error", err)
	}
}

// WriteJSONResponse writes a JSON response
func WriteJSONResponse(w http.ResponseWriter, data interface{}, statusCode int) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	return json.NewEncoder(w).Encode(data)
}

// NewAppError creates a new application error
func NewAppError(code, message string) AppError {
	return AppError{
		Code:    code,
		Message: message,
	}
}
