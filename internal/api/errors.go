package api

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
)

// ErrorResponse represents a standardised error response
type ErrorResponse struct {
	Status    int    `json:"status"`
	Message   string `json:"message"`
	Code      string `json:"code,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// ErrorCode represents standard error codes
type ErrorCode string

const (
	// Client errors (4xx)
	ErrCodeBadRequest     ErrorCode = "BAD_REQUEST"
	ErrCodeUnauthorised   ErrorCode = "UNAUTHORISED"
	ErrCodeForbidden      ErrorCode = "FORBIDDEN"
	ErrCodeNotFound       ErrorCode = "NOT_FOUND"
	ErrCodeMethodNotAllowed ErrorCode = "METHOD_NOT_ALLOWED"
	ErrCodeConflict       ErrorCode = "CONFLICT"
	ErrCodeValidation     ErrorCode = "VALIDATION_ERROR"
	ErrCodeRateLimit      ErrorCode = "RATE_LIMIT_EXCEEDED"

	// Server errors (5xx)
	ErrCodeInternal       ErrorCode = "INTERNAL_ERROR"
	ErrCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
	ErrCodeDatabaseError  ErrorCode = "DATABASE_ERROR"
)

// WriteError writes a standardised error response
func WriteError(w http.ResponseWriter, r *http.Request, err error, status int, code ErrorCode) {
	requestID := GetRequestID(r)
	
	errResp := ErrorResponse{
		Status:    status,
		Message:   err.Error(),
		Code:      string(code),
		RequestID: requestID,
	}

	// Log the error with context
	log.Error().
		Err(err).
		Str("request_id", requestID).
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Int("status", status).
		Str("code", string(code)).
		Msg("API error response")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errResp)
}

// WriteErrorMessage writes a standardised error response with a custom message
func WriteErrorMessage(w http.ResponseWriter, r *http.Request, message string, status int, code ErrorCode) {
	requestID := GetRequestID(r)
	
	errResp := ErrorResponse{
		Status:    status,
		Message:   message,
		Code:      string(code),
		RequestID: requestID,
	}

	// Log the error with context
	log.Error().
		Str("request_id", requestID).
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Int("status", status).
		Str("code", string(code)).
		Str("message", message).
		Msg("API error response")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errResp)
}

// Common error helpers for frequent use cases

// BadRequest responds with a 400 Bad Request error
func BadRequest(w http.ResponseWriter, r *http.Request, message string) {
	WriteErrorMessage(w, r, message, http.StatusBadRequest, ErrCodeBadRequest)
}

// Unauthorised responds with a 401 Unauthorised error
func Unauthorised(w http.ResponseWriter, r *http.Request, message string) {
	WriteErrorMessage(w, r, message, http.StatusUnauthorized, ErrCodeUnauthorised)
}

// Forbidden responds with a 403 Forbidden error
func Forbidden(w http.ResponseWriter, r *http.Request, message string) {
	WriteErrorMessage(w, r, message, http.StatusForbidden, ErrCodeForbidden)
}

// NotFound responds with a 404 Not Found error
func NotFound(w http.ResponseWriter, r *http.Request, message string) {
	WriteErrorMessage(w, r, message, http.StatusNotFound, ErrCodeNotFound)
}

// MethodNotAllowed responds with a 405 Method Not Allowed error
func MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	WriteErrorMessage(w, r, "Method not allowed", http.StatusMethodNotAllowed, ErrCodeMethodNotAllowed)
}

// InternalError responds with a 500 Internal Server Error
func InternalError(w http.ResponseWriter, r *http.Request, err error) {
	WriteError(w, r, err, http.StatusInternalServerError, ErrCodeInternal)
}

// DatabaseError responds with a 500 error for database issues
func DatabaseError(w http.ResponseWriter, r *http.Request, err error) {
	WriteError(w, r, err, http.StatusInternalServerError, ErrCodeDatabaseError)
}

// ServiceUnavailable responds with a 503 Service Unavailable error
func ServiceUnavailable(w http.ResponseWriter, r *http.Request, message string) {
	WriteErrorMessage(w, r, message, http.StatusServiceUnavailable, ErrCodeServiceUnavailable)
}