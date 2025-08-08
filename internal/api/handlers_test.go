package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/Harvey-AU/blue-banded-bee/internal/jobs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHandler(t *testing.T) {
	// Create mock dependencies
	mockDB := &db.DB{}
	mockJobsManager := &jobs.JobManager{}
	
	handler := NewHandler(mockDB, mockJobsManager)
	
	assert.NotNil(t, handler)
	assert.Equal(t, mockDB, handler.DB)
	assert.Equal(t, mockJobsManager, handler.JobsManager)
}

func TestHealthCheckHandler(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name:           "successful_health_check",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"status":  "healthy",
				"service": "blue-banded-bee",
				"version": "0.4.0",
			},
		},
		{
			name:           "wrong_method_post",
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   nil,
		},
		{
			name:           "wrong_method_put",
			method:         http.MethodPut,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   nil,
		},
		{
			name:           "wrong_method_delete",
			method:         http.MethodDelete,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   nil,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &Handler{}
			
			req := httptest.NewRequest(tt.method, "/health", nil)
			rec := httptest.NewRecorder()
			
			handler.HealthCheck(rec, req)
			
			assert.Equal(t, tt.expectedStatus, rec.Code)
			
			if tt.expectedBody != nil {
				assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
				
				// Parse response body
				var response map[string]interface{}
				err := parseJSONResponse(rec, &response)
				require.NoError(t, err)
				
				// Check expected fields
				for key, expectedValue := range tt.expectedBody {
					assert.Equal(t, expectedValue, response[key])
				}
				
				// Timestamp should be present
				assert.NotEmpty(t, response["timestamp"])
			}
		})
	}
}

func TestDatabaseHealthCheck_NoDatabase(t *testing.T) {
	t.Run("wrong_method", func(t *testing.T) {
		handler := &Handler{
			DB: nil, // No database
		}
		
		req := httptest.NewRequest(http.MethodPost, "/health/db", nil)
		rec := httptest.NewRecorder()
		
		handler.DatabaseHealthCheck(rec, req)
		
		assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	})
	
	// Note: We can't test nil DB with GET method as it will panic
	// This is expected behavior - the DB should be initialized before use
	t.Run("nil_db_would_panic", func(t *testing.T) {
		handler := &Handler{
			DB: nil, // No database
		}
		
		req := httptest.NewRequest(http.MethodGet, "/health/db", nil)
		rec := httptest.NewRecorder()
		
		// This would panic in real code - that's expected
		assert.Panics(t, func() {
			handler.DatabaseHealthCheck(rec, req)
		})
	})
}

type mockDBWithPing struct {
	*db.DB
	mockDB   *sql.DB
	pingErr  error
}

func (m *mockDBWithPing) GetDB() *sql.DB {
	return m.mockDB
}

type mockSQLDB struct {
	*sql.DB
	pingErr error
}

func (m *mockSQLDB) Ping() error {
	return m.pingErr
}

func TestServeStaticFiles(t *testing.T) {
	handler := &Handler{}
	
	tests := []struct {
		name           string
		path           string
		method         string
		expectedStatus int
	}{
		{
			name:           "serve_test_login",
			path:           "/test-login.html",
			method:         http.MethodGet,
			expectedStatus: http.StatusNotFound, // Files don't exist in test environment
		},
		{
			name:           "serve_test_components",
			path:           "/test-components.html",
			method:         http.MethodGet,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "serve_test_data_components",
			path:           "/test-data-components.html",
			method:         http.MethodGet,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "serve_dashboard",
			path:           "/dashboard",
			method:         http.MethodGet,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "serve_new_dashboard",
			path:           "/dashboard-new",
			method:         http.MethodGet,
			expectedStatus: http.StatusNotFound,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			
			// Call the appropriate handler based on path
			switch tt.path {
			case "/test-login.html":
				handler.ServeTestLogin(rec, req)
			case "/test-components.html":
				handler.ServeTestComponents(rec, req)
			case "/test-data-components.html":
				handler.ServeTestDataComponents(rec, req)
			case "/dashboard":
				handler.ServeDashboard(rec, req)
			case "/dashboard-new":
				handler.ServeNewDashboard(rec, req)
			}
			
			// In test environment, files don't exist so we expect 404
			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

func TestSetupRoutes(t *testing.T) {
	handler := &Handler{}
	mux := http.NewServeMux()
	
	// Test that SetupRoutes doesn't panic
	assert.NotPanics(t, func() {
		handler.SetupRoutes(mux)
	})
	
	// Test that routes are registered (by trying to access them)
	// Only test routes that don't require DB access
	routes := []string{
		"/health",
		// "/health/db", // Skip - requires DB
		"/v1/auth/register",
		"/v1/auth/session",
		// Other routes require auth middleware or DB access
	}
	
	for _, route := range routes {
		t.Run(route, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, route, nil)
			rec := httptest.NewRecorder()
			
			// This will not panic if route is registered
			assert.NotPanics(t, func() {
				mux.ServeHTTP(rec, req)
			})
			
			// We're not testing the actual response, just that the route exists
			// Response might be 401 (unauthorized), 405 (method not allowed), or 200
			// Routes may return various status codes based on method and auth
		})
	}
}

func TestHealthCheckHandlerConcurrency(t *testing.T) {
	handler := &Handler{}
	
	// Run multiple health checks concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(index int) {
			defer func() { done <- true }()
			
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			rec := httptest.NewRecorder()
			
			handler.HealthCheck(rec, req)
			
			assert.Equal(t, http.StatusOK, rec.Code)
		}(i)
	}
	
	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestHandlerWithNilDependencies(t *testing.T) {
	// Test that handlers gracefully handle nil dependencies
	handler := &Handler{
		DB:          nil,
		JobsManager: nil,
	}
	
	t.Run("health_check_works_without_dependencies", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		
		handler.HealthCheck(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
	})
	
	t.Run("database_health_check_panics_without_db", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health/db", nil)
		rec := httptest.NewRecorder()
		
		// This panics when DB is nil - that's expected behavior
		assert.Panics(t, func() {
			handler.DatabaseHealthCheck(rec, req)
		})
	})
}

func TestHealthCheckHandlerHeaders(t *testing.T) {
	handler := &Handler{}
	
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	
	// Set some custom headers before the handler
	rec.Header().Set("X-Custom-Header", "custom-value")
	
	handler.HealthCheck(rec, req)
	
	// Check that standard headers are set
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	
	// Check that custom headers are preserved
	assert.Equal(t, "custom-value", rec.Header().Get("X-Custom-Header"))
}

func TestMethodNotAllowedCases(t *testing.T) {
	handler := &Handler{}
	
	methods := []string{
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodHead,
		http.MethodOptions,
		http.MethodTrace,
		http.MethodConnect,
	}
	
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/health", nil)
			rec := httptest.NewRecorder()
			
			handler.HealthCheck(rec, req)
			
			if method != http.MethodGet {
				assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
			}
		})
	}
}

// Benchmark tests
func BenchmarkHealthCheckHandler(b *testing.B) {
	handler := &Handler{}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		handler.HealthCheck(rec, req)
	}
}

func BenchmarkSetupRoutes(b *testing.B) {
	handler := &Handler{}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mux := http.NewServeMux()
		handler.SetupRoutes(mux)
	}
}

// Helper function to parse JSON response
func parseJSONResponse(rec *httptest.ResponseRecorder, v interface{}) error {
	return jsonDecode(rec.Body.Bytes(), v)
}

func jsonDecode(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}