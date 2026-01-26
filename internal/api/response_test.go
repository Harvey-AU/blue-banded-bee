package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestWriteJSON verifies basic JSON response writing
func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	data := map[string]string{"message": "test"}

	WriteJSON(w, r, data, http.StatusOK)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var result map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, "test", result["message"])
}

// TestSuccessResponseStructure verifies success response format
func TestSuccessResponseStructure(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	data := map[string]string{"result": "ok"}

	WriteSuccess(w, r, data, "operation completed")

	assert.Equal(t, http.StatusOK, w.Code)

	var response SuccessResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify response structure
	assert.Equal(t, "success", response.Status)
	assert.Equal(t, "operation completed", response.Message)

	// Verify data is present
	dataMap := response.Data.(map[string]any)
	assert.Equal(t, "ok", dataMap["result"])
}

// TestHealthResponseStructure verifies health check response format
func TestHealthResponseStructure(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/health", nil)

	WriteHealthy(w, r, "blue-banded-bee", "1.0.0")

	assert.Equal(t, http.StatusOK, w.Code)

	var response HealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, "healthy", response.Status)
	assert.Equal(t, "blue-banded-bee", response.Service)
	assert.Equal(t, "1.0.0", response.Version)
	assert.NotEmpty(t, response.Timestamp)
}
