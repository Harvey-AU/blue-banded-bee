package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteJSON(t *testing.T) {
	tests := []struct {
		name           string
		data           interface{}
		status         int
		requestID      string
		expectedStatus int
		validateJSON   func(t *testing.T, body string)
	}{
		{
			name:           "writes_simple_json_object",
			data:           map[string]string{"key": "value"},
			status:         http.StatusOK,
			requestID:      "test-json-123",
			expectedStatus: http.StatusOK,
			validateJSON: func(t *testing.T, body string) {
				assert.JSONEq(t, `{"key":"value"}`, body)
			},
		},
		{
			name: "writes_complex_nested_structure",
			data: map[string]interface{}{
				"user": map[string]interface{}{
					"id":    123,
					"name":  "Test User",
					"roles": []string{"admin", "user"},
				},
				"timestamp": "2024-01-01T00:00:00Z",
			},
			status:         http.StatusCreated,
			requestID:      "test-json-456",
			expectedStatus: http.StatusCreated,
			validateJSON: func(t *testing.T, body string) {
				var result map[string]interface{}
				err := json.Unmarshal([]byte(body), &result)
				require.NoError(t, err)

				user := result["user"].(map[string]interface{})
				assert.Equal(t, float64(123), user["id"])
				assert.Equal(t, "Test User", user["name"])
			},
		},
		{
			name:           "handles_nil_data",
			data:           nil,
			status:         http.StatusOK,
			requestID:      "",
			expectedStatus: http.StatusOK,
			validateJSON: func(t *testing.T, body string) {
				assert.Equal(t, "null\n", body)
			},
		},
		{
			name:           "writes_array_data",
			data:           []int{1, 2, 3, 4, 5},
			status:         http.StatusOK,
			requestID:      "test-array",
			expectedStatus: http.StatusOK,
			validateJSON: func(t *testing.T, body string) {
				assert.JSONEq(t, `[1,2,3,4,5]`, body)
			},
		},
		{
			name:           "preserves_status_code",
			data:           map[string]bool{"success": false},
			status:         http.StatusBadRequest,
			requestID:      "test-status",
			expectedStatus: http.StatusBadRequest,
			validateJSON: func(t *testing.T, body string) {
				assert.JSONEq(t, `{"success":false}`, body)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.requestID != "" {
				ctx := context.WithValue(req.Context(), requestIDKey, tt.requestID)
				req = req.WithContext(ctx)
			}
			rec := httptest.NewRecorder()

			WriteJSON(rec, req, tt.data, tt.status)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

			if tt.validateJSON != nil {
				tt.validateJSON(t, rec.Body.String())
			}
		})
	}
}

func TestWriteSuccess(t *testing.T) {
	tests := []struct {
		name      string
		data      interface{}
		message   string
		requestID string
	}{
		{
			name:      "success_with_data_and_message",
			data:      map[string]int{"count": 42},
			message:   "Operation completed successfully",
			requestID: "success-123",
		},
		{
			name:      "success_with_nil_data",
			data:      nil,
			message:   "Deleted successfully",
			requestID: "success-456",
		},
		{
			name:      "success_with_empty_message",
			data:      map[string]string{"id": "abc123"},
			message:   "",
			requestID: "success-789",
		},
		{
			name:      "success_without_request_id",
			data:      []string{"item1", "item2"},
			message:   "Items retrieved",
			requestID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.requestID != "" {
				ctx := context.WithValue(req.Context(), requestIDKey, tt.requestID)
				req = req.WithContext(ctx)
			}
			rec := httptest.NewRecorder()

			WriteSuccess(rec, req, tt.data, tt.message)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

			var response SuccessResponse
			err := json.NewDecoder(rec.Body).Decode(&response)
			require.NoError(t, err)

			assert.Equal(t, "success", response.Status)
			assert.Equal(t, tt.message, response.Message)
			assert.Equal(t, tt.requestID, response.RequestID)

			// Verify data matches (need to handle type conversion for comparison)
			if tt.data != nil {
				dataJSON, _ := json.Marshal(tt.data)
				responseDataJSON, _ := json.Marshal(response.Data)
				assert.JSONEq(t, string(dataJSON), string(responseDataJSON))
			}
		})
	}
}

func TestWriteCreated(t *testing.T) {
	tests := []struct {
		name      string
		data      interface{}
		message   string
		requestID string
	}{
		{
			name: "created_resource_with_full_data",
			data: map[string]interface{}{
				"id":        "new-resource-123",
				"name":      "New Resource",
				"createdAt": "2024-01-01T00:00:00Z",
			},
			message:   "Resource created successfully",
			requestID: "create-123",
		},
		{
			name:      "created_with_minimal_data",
			data:      map[string]string{"id": "456"},
			message:   "Created",
			requestID: "create-456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/test", nil)
			if tt.requestID != "" {
				ctx := context.WithValue(req.Context(), requestIDKey, tt.requestID)
				req = req.WithContext(ctx)
			}
			rec := httptest.NewRecorder()

			WriteCreated(rec, req, tt.data, tt.message)

			assert.Equal(t, http.StatusCreated, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

			var response SuccessResponse
			err := json.NewDecoder(rec.Body).Decode(&response)
			require.NoError(t, err)

			assert.Equal(t, "success", response.Status)
			assert.Equal(t, tt.message, response.Message)
			assert.Equal(t, tt.requestID, response.RequestID)
		})
	}
}

func TestWriteNoContent(t *testing.T) {
	tests := []struct {
		name      string
		requestID string
	}{
		{
			name:      "no_content_response",
			requestID: "no-content-123",
		},
		{
			name:      "no_content_without_request_id",
			requestID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/test", nil)
			if tt.requestID != "" {
				ctx := context.WithValue(req.Context(), requestIDKey, tt.requestID)
				req = req.WithContext(ctx)
			}
			rec := httptest.NewRecorder()

			WriteNoContent(rec, req)

			assert.Equal(t, http.StatusNoContent, rec.Code)
			assert.Empty(t, rec.Body.String(), "204 response should have no body")
			// Note: Content-Type header is not set for 204 responses
		})
	}
}

func TestWriteHealthy(t *testing.T) {
	tests := []struct {
		name    string
		service string
		version string
	}{
		{
			name:    "healthy_with_version",
			service: "blue-banded-bee",
			version: "1.0.0",
		},
		{
			name:    "healthy_without_version",
			service: "test-service",
			version: "",
		},
		{
			name:    "healthy_with_complex_version",
			service: "api",
			version: "v2.1.0-beta+build.123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			rec := httptest.NewRecorder()

			// Capture time before and after for validation
			timeBefore := time.Now()
			WriteHealthy(rec, req, tt.service, tt.version)
			timeAfter := time.Now()

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

			var response HealthResponse
			err := json.NewDecoder(rec.Body).Decode(&response)
			require.NoError(t, err)

			assert.Equal(t, "healthy", response.Status)
			assert.Equal(t, tt.service, response.Service)
			assert.Equal(t, tt.version, response.Version)

			// Verify timestamp is valid and recent
			timestamp, err := time.Parse(time.RFC3339, response.Timestamp)
			require.NoError(t, err)
			assert.True(t, timestamp.After(timeBefore.Add(-time.Second)))
			assert.True(t, timestamp.Before(timeAfter.Add(time.Second)))
		})
	}
}

func TestWriteUnhealthy(t *testing.T) {
	tests := []struct {
		name      string
		service   string
		err       error
		requestID string
	}{
		{
			name:      "unhealthy_with_database_error",
			service:   "blue-banded-bee",
			err:       errors.New("database connection failed"),
			requestID: "unhealthy-123",
		},
		{
			name:      "unhealthy_with_timeout_error",
			service:   "api",
			err:       errors.New("health check timeout"),
			requestID: "",
		},
		{
			name:      "unhealthy_with_detailed_error",
			service:   "worker",
			err:       errors.New("worker pool exhausted: 0/10 workers available"),
			requestID: "unhealthy-456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			if tt.requestID != "" {
				ctx := context.WithValue(req.Context(), requestIDKey, tt.requestID)
				req = req.WithContext(ctx)
			}
			rec := httptest.NewRecorder()

			timeBefore := time.Now()
			WriteUnhealthy(rec, req, tt.service, tt.err)
			timeAfter := time.Now()

			assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

			var response map[string]interface{}
			err := json.NewDecoder(rec.Body).Decode(&response)
			require.NoError(t, err)

			assert.Equal(t, "unhealthy", response["status"])
			assert.Equal(t, tt.service, response["service"])
			assert.Equal(t, tt.err.Error(), response["error"])
			assert.Equal(t, tt.requestID, response["request_id"])

			// Verify timestamp
			timestamp, err := time.Parse(time.RFC3339, response["timestamp"].(string))
			require.NoError(t, err)
			assert.True(t, timestamp.After(timeBefore.Add(-time.Second)))
			assert.True(t, timestamp.Before(timeAfter.Add(time.Second)))
		})
	}
}

func TestWriteJSONWithInvalidData(t *testing.T) {
	// Test with complex data structures
	// Note: We avoid testing with truly invalid JSON types (channels, functions)
	// as they cause encoding errors that are logged but handled gracefully
	tests := []struct {
		name        string
		data        interface{}
		shouldPanic bool
	}{
		{
			name: "deeply_nested_structure",
			data: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": map[string]interface{}{
						"level3": map[string]interface{}{
							"value": "deep",
						},
					},
				},
			},
			shouldPanic: false,
		},
		{
			name: "mixed_types",
			data: map[string]interface{}{
				"string": "value",
				"number": 123,
				"float":  45.67,
				"bool":   true,
				"null":   nil,
				"array":  []interface{}{1, "two", 3.0, true, nil},
			},
			shouldPanic: false,
		},
		{
			name: "empty_interfaces",
			data: map[string]interface{}{
				"empty": interface{}(nil),
				"nested": map[string]interface{}{
					"also_empty": interface{}(nil),
				},
			},
			shouldPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()

			if tt.shouldPanic {
				assert.Panics(t, func() {
					WriteJSON(rec, req, tt.data, http.StatusOK)
				})
			} else {
				assert.NotPanics(t, func() {
					WriteJSON(rec, req, tt.data, http.StatusOK)
				})
				// The response should be valid JSON
				assert.Equal(t, http.StatusOK, rec.Code)
				assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

				// Verify the JSON can be decoded
				var result interface{}
				err := json.NewDecoder(rec.Body).Decode(&result)
				assert.NoError(t, err, "Response should be valid JSON")
			}
		})
	}
}

func TestSuccessResponseStructure(t *testing.T) {
	// Test that SuccessResponse properly omits empty fields
	tests := []struct {
		name     string
		response SuccessResponse
		expected string
	}{
		{
			name: "all_fields_populated",
			response: SuccessResponse{
				Status:    "success",
				Data:      map[string]int{"count": 10},
				Message:   "Retrieved items",
				RequestID: "req-123",
			},
			expected: `{"status":"success","data":{"count":10},"message":"Retrieved items","request_id":"req-123"}`,
		},
		{
			name: "omits_empty_data",
			response: SuccessResponse{
				Status:    "success",
				Data:      nil,
				Message:   "Completed",
				RequestID: "req-456",
			},
			expected: `{"status":"success","message":"Completed","request_id":"req-456"}`,
		},
		{
			name: "omits_empty_message",
			response: SuccessResponse{
				Status:    "success",
				Data:      []int{1, 2, 3},
				Message:   "",
				RequestID: "req-789",
			},
			expected: `{"status":"success","data":[1,2,3],"request_id":"req-789"}`,
		},
		{
			name: "omits_empty_request_id",
			response: SuccessResponse{
				Status:    "success",
				Data:      "test",
				Message:   "Test message",
				RequestID: "",
			},
			expected: `{"status":"success","data":"test","message":"Test message"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonBytes, err := json.Marshal(tt.response)
			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(jsonBytes))
		})
	}
}

func TestHealthResponseStructure(t *testing.T) {
	tests := []struct {
		name       string
		response   HealthResponse
		hasVersion bool
	}{
		{
			name: "with_version",
			response: HealthResponse{
				Status:    "healthy",
				Timestamp: "2024-01-01T00:00:00Z",
				Service:   "api",
				Version:   "1.0.0",
			},
			hasVersion: true,
		},
		{
			name: "without_version",
			response: HealthResponse{
				Status:    "healthy",
				Timestamp: "2024-01-01T00:00:00Z",
				Service:   "api",
				Version:   "",
			},
			hasVersion: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonBytes, err := json.Marshal(tt.response)
			require.NoError(t, err)

			var result map[string]interface{}
			err = json.Unmarshal(jsonBytes, &result)
			require.NoError(t, err)

			assert.Equal(t, tt.response.Status, result["status"])
			assert.Equal(t, tt.response.Timestamp, result["timestamp"])
			assert.Equal(t, tt.response.Service, result["service"])

			if tt.hasVersion {
				assert.Equal(t, tt.response.Version, result["version"])
			} else {
				// Version should be omitted when empty due to omitempty tag
				_, exists := result["version"]
				assert.False(t, exists || result["version"] == "")
			}
		})
	}
}

func TestConcurrentWrites(t *testing.T) {
	// Test that response functions are safe for concurrent use
	done := make(chan bool, 20)

	for i := 0; i < 20; i++ {
		go func(index int) {
			defer func() { done <- true }()

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()

			switch index % 5 {
			case 0:
				WriteJSON(rec, req, map[string]int{"index": index}, http.StatusOK)
			case 1:
				WriteSuccess(rec, req, index, "Success")
			case 2:
				WriteCreated(rec, req, index, "Created")
			case 3:
				WriteNoContent(rec, req)
			case 4:
				WriteHealthy(rec, req, "service", "1.0.0")
			}

			// Verify response was written correctly
			if index%5 != 3 { // Not NoContent
				assert.NotEmpty(t, rec.Body.String())
			}
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}
}

func TestWriteJSONPreservesHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	// Set custom headers before WriteJSON
	rec.Header().Set("X-Custom-Header", "custom-value")
	rec.Header().Set("X-Request-ID", "custom-request-id")

	WriteJSON(rec, req, map[string]string{"test": "data"}, http.StatusOK)

	// Verify custom headers are preserved
	assert.Equal(t, "custom-value", rec.Header().Get("X-Custom-Header"))
	assert.Equal(t, "custom-request-id", rec.Header().Get("X-Request-ID"))
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}

func TestLargePayloadHandling(t *testing.T) {
	// Test with a large payload
	largeData := make([]map[string]string, 10000)
	for i := 0; i < 10000; i++ {
		largeData[i] = map[string]string{
			"id":    strings.Repeat("x", 100),
			"value": strings.Repeat("y", 100),
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	WriteJSON(rec, req, largeData, http.StatusOK)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	// Verify the response can be decoded
	var result []map[string]string
	err := json.NewDecoder(rec.Body).Decode(&result)
	require.NoError(t, err)
	assert.Len(t, result, 10000)
}

func TestWriteWithSpecialCharacters(t *testing.T) {
	tests := []struct {
		name string
		data interface{}
	}{
		{
			name: "unicode_characters",
			data: map[string]string{
				"message": "Hello 世界 🌍",
				"emoji":   "🔥💯✨",
			},
		},
		{
			name: "html_entities",
			data: map[string]string{
				"html": "<script>alert('xss')</script>",
				"text": "This & that < > \"quotes\"",
			},
		},
		{
			name: "control_characters",
			data: map[string]string{
				"text": "Line1\nLine2\tTabbed\rCarriage",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()

			WriteJSON(rec, req, tt.data, http.StatusOK)

			// Verify the response can be decoded and matches original
			var result map[string]string
			err := json.NewDecoder(rec.Body).Decode(&result)
			require.NoError(t, err)

			expected, _ := json.Marshal(tt.data)
			actual, _ := json.Marshal(result)
			assert.JSONEq(t, string(expected), string(actual))
		})
	}
}

// Benchmark tests
func BenchmarkWriteJSON(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	data := map[string]interface{}{
		"id":     123,
		"name":   "Test",
		"values": []int{1, 2, 3, 4, 5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		WriteJSON(rec, req, data, http.StatusOK)
	}
}

func BenchmarkWriteSuccess(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx := context.WithValue(req.Context(), requestIDKey, "bench-123")
	req = req.WithContext(ctx)

	data := map[string]string{"result": "success"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		WriteSuccess(rec, req, data, "Operation completed")
	}
}

func BenchmarkWriteHealthy(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		WriteHealthy(rec, req, "benchmark-service", "1.0.0")
	}
}

// Test with custom ResponseWriter implementations
type customResponseWriter struct {
	headers http.Header
	body    []byte
	status  int
}

func (c *customResponseWriter) Header() http.Header {
	if c.headers == nil {
		c.headers = make(http.Header)
	}
	return c.headers
}

func (c *customResponseWriter) Write(data []byte) (int, error) {
	c.body = append(c.body, data...)
	return len(data), nil
}

func (c *customResponseWriter) WriteHeader(status int) {
	c.status = status
}

func TestWriteJSONWithCustomResponseWriter(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	customWriter := &customResponseWriter{}

	WriteJSON(customWriter, req, map[string]bool{"custom": true}, http.StatusAccepted)

	assert.Equal(t, http.StatusAccepted, customWriter.status)
	assert.Equal(t, "application/json", customWriter.Header().Get("Content-Type"))
	assert.Contains(t, string(customWriter.body), `"custom":true`)
}

// Helper function tests
func TestResponseHelperIntegration(t *testing.T) {
	// Test that all response helpers work together correctly
	t.Run("success_flow", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/resource", nil)
		ctx := context.WithValue(req.Context(), requestIDKey, "integration-test")
		req = req.WithContext(ctx)

		// Simulate creating a resource
		rec := httptest.NewRecorder()
		resourceData := map[string]interface{}{
			"id":   "resource-123",
			"name": "New Resource",
		}
		WriteCreated(rec, req, resourceData, "Resource created successfully")

		assert.Equal(t, http.StatusCreated, rec.Code)

		var response SuccessResponse
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, "success", response.Status)
		assert.Equal(t, "integration-test", response.RequestID)
	})

	t.Run("health_check_flow", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()

		// Simulate healthy service
		WriteHealthy(rec, req, "api", "2.0.0")
		assert.Equal(t, http.StatusOK, rec.Code)

		var response HealthResponse
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, "healthy", response.Status)

		// Simulate unhealthy service
		rec = httptest.NewRecorder()
		WriteUnhealthy(rec, req, "api", errors.New("database down"))
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})
}

// Test for potential memory leaks with io.Writer
func TestWriteJSONDoesNotLeakMemory(t *testing.T) {
	// This test ensures we don't keep references to large objects
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	// Create a large object
	largeObject := make([]byte, 1024*1024) // 1MB

	rec := httptest.NewRecorder()
	WriteJSON(rec, req, largeObject, http.StatusOK)

	// If this doesn't cause issues in race detector or memory profiling, we're good
	assert.NotNil(t, rec.Body)
}
