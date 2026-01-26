package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestWriteError verifies error response structure
func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	WriteError(w, r, errors.New("invalid input"), http.StatusBadRequest, ErrCodeBadRequest)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, "BAD_REQUEST", response.Code)
	assert.Equal(t, "invalid input", response.Message)
	assert.Equal(t, http.StatusBadRequest, response.Status)
}

// TestErrorCodesConstants verifies error code constants are correctly defined
func TestErrorCodesConstants(t *testing.T) {
	assert.Equal(t, ErrorCode("BAD_REQUEST"), ErrCodeBadRequest)
	assert.Equal(t, ErrorCode("UNAUTHORISED"), ErrCodeUnauthorised)
	assert.Equal(t, ErrorCode("FORBIDDEN"), ErrCodeForbidden)
	assert.Equal(t, ErrorCode("NOT_FOUND"), ErrCodeNotFound)
	assert.Equal(t, ErrorCode("INTERNAL_ERROR"), ErrCodeInternal)
}

// TestBadRequest verifies BadRequest helper
func TestBadRequest(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	BadRequest(w, r, "validation failed")

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, "BAD_REQUEST", response.Code)
	assert.Equal(t, "validation failed", response.Message)
}

// TestUnauthorised verifies Unauthorised helper
func TestUnauthorised(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	Unauthorised(w, r, "invalid token")

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, "UNAUTHORISED", response.Code)
	assert.Equal(t, "invalid token", response.Message)
}
