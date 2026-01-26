package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestErrors verifies all error response types using table-driven tests
func TestErrors(t *testing.T) {
	tests := []struct {
		name           string
		testFunc       func(*httptest.ResponseRecorder, *http.Request)
		expectedStatus int
		expectedCode   string
		expectedMsg    string
	}{
		{
			name: "bad_request",
			testFunc: func(w *httptest.ResponseRecorder, r *http.Request) {
				BadRequest(w, r, "validation failed")
			},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "BAD_REQUEST",
			expectedMsg:    "validation failed",
		},
		{
			name: "unauthorised",
			testFunc: func(w *httptest.ResponseRecorder, r *http.Request) {
				Unauthorised(w, r, "invalid token")
			},
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "UNAUTHORISED",
			expectedMsg:    "invalid token",
		},
		{
			name: "internal_error",
			testFunc: func(w *httptest.ResponseRecorder, r *http.Request) {
				WriteError(w, r, errors.New("database connection failed"), http.StatusInternalServerError, ErrCodeInternal)
			},
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   "INTERNAL_ERROR",
			expectedMsg:    "database connection failed",
		},
		{
			name: "write_error_with_custom_code",
			testFunc: func(w *httptest.ResponseRecorder, r *http.Request) {
				WriteError(w, r, errors.New("invalid input"), http.StatusBadRequest, ErrCodeBadRequest)
			},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "BAD_REQUEST",
			expectedMsg:    "invalid input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/test", nil)

			tt.testFunc(w, r)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response ErrorResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			assert.Equal(t, tt.expectedCode, response.Code)
			assert.Equal(t, tt.expectedMsg, response.Message)
			assert.Equal(t, tt.expectedStatus, response.Status)
		})
	}
}

// TestErrorCodesConstants verifies error code constants are correctly defined
func TestErrorCodesConstants(t *testing.T) {
	assert.Equal(t, ErrorCode("BAD_REQUEST"), ErrCodeBadRequest)
	assert.Equal(t, ErrorCode("UNAUTHORISED"), ErrCodeUnauthorised)
	assert.Equal(t, ErrorCode("FORBIDDEN"), ErrCodeForbidden)
	assert.Equal(t, ErrorCode("NOT_FOUND"), ErrCodeNotFound)
	assert.Equal(t, ErrorCode("INTERNAL_ERROR"), ErrCodeInternal)
}
