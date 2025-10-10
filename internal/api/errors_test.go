package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		status         int
		code           ErrorCode
		requestID      string
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "writes internal server error",
			err:            errors.New("database connection failed"),
			status:         http.StatusInternalServerError,
			code:           ErrCodeDatabaseError,
			requestID:      "test-request-123",
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   "DATABASE_ERROR",
		},
		{
			name:           "writes bad request error",
			err:            errors.New("invalid input"),
			status:         http.StatusBadRequest,
			code:           ErrCodeValidation,
			requestID:      "test-request-456",
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "VALIDATION_ERROR",
		},
		{
			name:           "handles missing request ID",
			err:            errors.New("test error"),
			status:         http.StatusNotFound,
			code:           ErrCodeNotFound,
			requestID:      "",
			expectedStatus: http.StatusNotFound,
			expectedCode:   "NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test request and response recorder
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.requestID != "" {
				ctx := context.WithValue(req.Context(), requestIDKey, tt.requestID)
				req = req.WithContext(ctx)
			}
			rec := httptest.NewRecorder()

			// Call the function
			WriteError(rec, req, tt.err, tt.status, tt.code)

			// Check status code
			assert.Equal(t, tt.expectedStatus, rec.Code)

			// Check content type header
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

			// Parse response body
			var response ErrorResponse
			err := json.NewDecoder(rec.Body).Decode(&response)
			require.NoError(t, err)

			// Check response fields
			assert.Equal(t, tt.expectedStatus, response.Status)
			assert.Equal(t, tt.err.Error(), response.Message)
			assert.Equal(t, tt.expectedCode, response.Code)
			assert.Equal(t, tt.requestID, response.RequestID)
		})
	}
}

func TestWriteErrorMessage(t *testing.T) {
	tests := []struct {
		name           string
		message        string
		status         int
		code           ErrorCode
		requestID      string
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "writes custom error message",
			message:        "Custom error message",
			status:         http.StatusForbidden,
			code:           ErrCodeForbidden,
			requestID:      "test-request-789",
			expectedStatus: http.StatusForbidden,
			expectedCode:   "FORBIDDEN",
		},
		{
			name:           "writes rate limit error",
			message:        "Too many requests",
			status:         http.StatusTooManyRequests,
			code:           ErrCodeRateLimit,
			requestID:      "test-request-abc",
			expectedStatus: http.StatusTooManyRequests,
			expectedCode:   "RATE_LIMIT_EXCEEDED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test request and response recorder
			req := httptest.NewRequest(http.MethodPost, "/test", nil)
			if tt.requestID != "" {
				ctx := context.WithValue(req.Context(), requestIDKey, tt.requestID)
				req = req.WithContext(ctx)
			}
			rec := httptest.NewRecorder()

			// Call the function
			WriteErrorMessage(rec, req, tt.message, tt.status, tt.code)

			// Check status code
			assert.Equal(t, tt.expectedStatus, rec.Code)

			// Check content type header
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

			// Parse response body
			var response ErrorResponse
			err := json.NewDecoder(rec.Body).Decode(&response)
			require.NoError(t, err)

			// Check response fields
			assert.Equal(t, tt.expectedStatus, response.Status)
			assert.Equal(t, tt.message, response.Message)
			assert.Equal(t, tt.expectedCode, response.Code)
			assert.Equal(t, tt.requestID, response.RequestID)
		})
	}
}

func TestBadRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	BadRequest(rec, req, "Invalid parameters")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var response ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, response.Status)
	assert.Equal(t, "Invalid parameters", response.Message)
	assert.Equal(t, "BAD_REQUEST", response.Code)
}

func TestUnauthorised(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	Unauthorised(rec, req, "Authentication required")

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var response ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, response.Status)
	assert.Equal(t, "Authentication required", response.Message)
	assert.Equal(t, "UNAUTHORISED", response.Code)
}

func TestForbidden(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	Forbidden(rec, req, "Access denied")

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var response ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, http.StatusForbidden, response.Status)
	assert.Equal(t, "Access denied", response.Message)
	assert.Equal(t, "FORBIDDEN", response.Code)
}

func TestNotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	NotFound(rec, req, "Resource not found")

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var response ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNotFound, response.Status)
	assert.Equal(t, "Resource not found", response.Message)
	assert.Equal(t, "NOT_FOUND", response.Code)
}

func TestMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	MethodNotAllowed(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var response ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, http.StatusMethodNotAllowed, response.Status)
	assert.Equal(t, "Method not allowed", response.Message)
	assert.Equal(t, "METHOD_NOT_ALLOWED", response.Code)
}

func TestInternalError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx := context.WithValue(req.Context(), requestIDKey, "test-internal-error")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	testErr := errors.New("internal server error occurred")
	InternalError(rec, req, testErr)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var response ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, http.StatusInternalServerError, response.Status)
	assert.Equal(t, "internal server error occurred", response.Message)
	assert.Equal(t, "INTERNAL_ERROR", response.Code)
	assert.Equal(t, "test-internal-error", response.RequestID)
}

func TestDatabaseError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx := context.WithValue(req.Context(), requestIDKey, "test-db-error")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	testErr := errors.New("connection pool exhausted")
	DatabaseError(rec, req, testErr)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var response ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, http.StatusInternalServerError, response.Status)
	assert.Equal(t, "connection pool exhausted", response.Message)
	assert.Equal(t, "DATABASE_ERROR", response.Code)
	assert.Equal(t, "test-db-error", response.RequestID)
}

func TestServiceUnavailable(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	ServiceUnavailable(rec, req, "Service temporarily unavailable")

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var response ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, http.StatusServiceUnavailable, response.Status)
	assert.Equal(t, "Service temporarily unavailable", response.Message)
	assert.Equal(t, "SERVICE_UNAVAILABLE", response.Code)
}

func TestErrorResponseJSONStructure(t *testing.T) {
	// Test that the JSON structure is correct
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx := context.WithValue(req.Context(), requestIDKey, "test-123")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	BadRequest(rec, req, "Test message")

	// Check that the JSON is properly formatted
	body := rec.Body.String()
	assert.True(t, strings.Contains(body, `"status":400`))
	assert.True(t, strings.Contains(body, `"message":"Test message"`))
	assert.True(t, strings.Contains(body, `"code":"BAD_REQUEST"`))
	assert.True(t, strings.Contains(body, `"request_id":"test-123"`))
}

func TestErrorCodesConstants(t *testing.T) {
	// Test that error code constants have expected values
	assert.Equal(t, ErrorCode("BAD_REQUEST"), ErrCodeBadRequest)
	assert.Equal(t, ErrorCode("UNAUTHORISED"), ErrCodeUnauthorised)
	assert.Equal(t, ErrorCode("FORBIDDEN"), ErrCodeForbidden)
	assert.Equal(t, ErrorCode("NOT_FOUND"), ErrCodeNotFound)
	assert.Equal(t, ErrorCode("METHOD_NOT_ALLOWED"), ErrCodeMethodNotAllowed)
	assert.Equal(t, ErrorCode("CONFLICT"), ErrCodeConflict)
	assert.Equal(t, ErrorCode("VALIDATION_ERROR"), ErrCodeValidation)
	assert.Equal(t, ErrorCode("RATE_LIMIT_EXCEEDED"), ErrCodeRateLimit)
	assert.Equal(t, ErrorCode("INTERNAL_ERROR"), ErrCodeInternal)
	assert.Equal(t, ErrorCode("SERVICE_UNAVAILABLE"), ErrCodeServiceUnavailable)
	assert.Equal(t, ErrorCode("DATABASE_ERROR"), ErrCodeDatabaseError)
}

func TestWriteErrorWithLargeMessage(t *testing.T) {
	// Test with a very long error message
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	longMessage := strings.Repeat("Error ", 1000)
	testErr := errors.New(longMessage)

	WriteError(rec, req, testErr, http.StatusBadRequest, ErrCodeValidation)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var response ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, longMessage, response.Message)
}

func TestWriteErrorPreservesExistingHeaders(t *testing.T) {
	// Test that existing headers are preserved
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	// Set a custom header before calling WriteError
	rec.Header().Set("X-Custom-Header", "custom-value")

	BadRequest(rec, req, "Test error")

	// Check that custom header is still present
	assert.Equal(t, "custom-value", rec.Header().Get("X-Custom-Header"))
	// And that Content-Type is also set
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}

func TestConcurrentErrorWrites(t *testing.T) {
	// Test that error handlers are safe for concurrent use
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(index int) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()

			switch index % 5 {
			case 0:
				BadRequest(rec, req, "Bad request")
			case 1:
				Unauthorised(rec, req, "Unauthorised")
			case 2:
				Forbidden(rec, req, "Forbidden")
			case 3:
				NotFound(rec, req, "Not found")
			case 4:
				InternalError(rec, req, errors.New("Internal error"))
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

// Benchmark tests
func BenchmarkWriteError(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	testErr := errors.New("test error")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		WriteError(rec, req, testErr, http.StatusBadRequest, ErrCodeBadRequest)
	}
}

func BenchmarkWriteErrorMessage(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		WriteErrorMessage(rec, req, "test message", http.StatusBadRequest, ErrCodeBadRequest)
	}
}

// Test helper to capture log output
func TestErrorLogging(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer

	// Test that errors are logged (this is more of a smoke test)
	req := httptest.NewRequest(http.MethodGet, "/test/path", nil)
	ctx := context.WithValue(req.Context(), requestIDKey, "log-test-123")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	// Call function that should log
	testErr := errors.New("test error for logging")
	WriteError(rec, req, testErr, http.StatusInternalServerError, ErrCodeInternal)

	// Verify the response was written correctly (logging is a side effect)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	// If we had access to the logger output, we could verify:
	// - Request ID is logged
	// - Method is logged
	// - Path is logged
	// - Status is logged
	// - Error code is logged
	_ = buf // Placeholder for potential future log capture
}
