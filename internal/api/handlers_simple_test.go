package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthResponse(t *testing.T) {
	// Test HealthResponse struct marshalling
	response := HealthResponse{
		Status:    "healthy",
		Service:   "test-service",
		Version:   "1.0.0",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.Marshal(response)
	require.NoError(t, err)

	var decoded HealthResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, response.Status, decoded.Status)
	assert.Equal(t, response.Service, decoded.Service)
	assert.Equal(t, response.Version, decoded.Version)
}

func TestCalculateDateRange(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name      string
		dateRange string
		expectNil bool
	}{
		{
			name:      "today",
			dateRange: "today",
			expectNil: false,
		},
		{
			name:      "last24",
			dateRange: "last24",
			expectNil: false,
		},
		{
			name:      "yesterday",
			dateRange: "yesterday",
			expectNil: false,
		},
		{
			name:      "last7",
			dateRange: "last7",
			expectNil: false,
		},
		{
			name:      "last30",
			dateRange: "last30",
			expectNil: false,
		},
		{
			name:      "last90",
			dateRange: "last90",
			expectNil: false,
		},
		{
			name:      "all",
			dateRange: "all",
			expectNil: true,
		},
		{
			name:      "unknown_defaults_to_last7",
			dateRange: "unknown",
			expectNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startDate, endDate := calculateDateRange(tt.dateRange, "UTC")

			if tt.expectNil {
				assert.Nil(t, startDate)
				assert.Nil(t, endDate)
			} else {
				assert.NotNil(t, startDate)
				assert.NotNil(t, endDate)

				if startDate != nil && endDate != nil {
					// For "today", start should be before or equal to end
					assert.True(t, !startDate.After(*endDate), "Start date should not be after end date")
					// End date should not be far in the future
					assert.True(t, !endDate.After(now.Add(24*time.Hour)), "End date should not be more than 24 hours in future")
				}
			}
		})
	}
}

func TestWebflowPayload(t *testing.T) {
	jsonData := `{
		"triggerType": "site_publish",
		"payload": {
			"domains": ["example.com", "www.example.com"],
			"publishedBy": {
				"displayName": "John Doe"
			}
		}
	}`

	var payload WebflowWebhookPayload
	err := json.Unmarshal([]byte(jsonData), &payload)
	require.NoError(t, err)

	assert.Equal(t, "site_publish", payload.TriggerType)
	assert.Len(t, payload.Payload.Domains, 2)
	assert.Equal(t, "example.com", payload.Payload.Domains[0])
	assert.Equal(t, "John Doe", payload.Payload.PublishedBy.DisplayName)
}

func TestStaticFileHandlers(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name         string
		handlerFunc  http.HandlerFunc
		url          string
		expectedFile string
	}{
		{
			name:         "ServeTestLogin",
			handlerFunc:  h.ServeTestLogin,
			url:          "/test-login.html",
			expectedFile: "test-login.html",
		},
		{
			name:         "ServeTestComponents",
			handlerFunc:  h.ServeTestComponents,
			url:          "/test-components.html",
			expectedFile: "test-components.html",
		},
		{
			name:         "ServeTestDataComponents",
			handlerFunc:  h.ServeTestDataComponents,
			url:          "/test-data-components.html",
			expectedFile: "test-data-components.html",
		},
		{
			name:         "ServeDashboard",
			handlerFunc:  h.ServeDashboard,
			url:          "/dashboard",
			expectedFile: "dashboard.html",
		},
		{
			name:         "ServeNewDashboard",
			handlerFunc:  h.ServeNewDashboard,
			url:          "/dashboard-new",
			expectedFile: "dashboard.html",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			rec := httptest.NewRecorder()

			// Call the handler
			tt.handlerFunc(rec, req)

			// When file doesn't exist, http.ServeFile returns 404
			// We can at least verify the response code
			if rec.Code != http.StatusOK {
				// Expected when files don't exist in test environment
				assert.Equal(t, http.StatusNotFound, rec.Code, "Should return 404 when file not found")
			} else {
				// If file exists, verify content type header is set
				contentType := rec.Header().Get("Content-Type")
				assert.NotEmpty(t, contentType, "Content-Type should be set when file exists")
			}
		})
	}
}

func TestHealthCheck(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name           string
		method         string
		expectedStatus int
	}{
		{
			name:           "GET_request",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST_not_allowed",
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "PUT_not_allowed",
			method:         http.MethodPut,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "DELETE_not_allowed",
			method:         http.MethodDelete,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/health", nil)
			rec := httptest.NewRecorder()

			h.HealthCheck(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)

			if tt.expectedStatus == http.StatusOK {
				var response HealthResponse
				err := json.NewDecoder(rec.Body).Decode(&response)
				require.NoError(t, err)

				assert.Equal(t, "healthy", response.Status)
				assert.Equal(t, "blue-banded-bee", response.Service)
				assert.Equal(t, Version, response.Version)
				assert.NotEmpty(t, response.Timestamp)
			}
		})
	}
}

// Benchmark tests
func BenchmarkCalculateDateRange(b *testing.B) {
	ranges := []string{"today", "last24", "yesterday", "last7", "last30", "last90", "all", "invalid"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rangeType := ranges[i%len(ranges)]
		calculateDateRange(rangeType, "UTC")
	}
}

func BenchmarkHealthResponseMarshalling(b *testing.B) {
	response := HealthResponse{
		Status:    "healthy",
		Service:   "benchmark-service",
		Version:   "1.0.0",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := json.Marshal(response)
		_ = data
	}
}

func BenchmarkWebflowPayloadParsing(b *testing.B) {
	jsonData := []byte(`{
		"triggerType": "site_publish",
		"payload": {
			"domains": ["example.com", "www.example.com"],
			"publishedBy": {
				"displayName": "John Doe"
			}
		}
	}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var payload WebflowWebhookPayload
		json.Unmarshal(jsonData, &payload)
	}
}
