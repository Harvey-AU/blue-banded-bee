package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDashboardSlowPagesIntegration(t *testing.T) {
	tests := []struct {
		name           string
		queryParams    string
		setupMocks     func(*MockDBClient, *MockJobManager)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:        "successful_slow_pages_default_range",
			queryParams: "",
			setupMocks: func(mockDB *MockDBClient, mockJM *MockJobManager) {
				mockDB.On("GetOrCreateUser", "test-user-123", "test@example.com", (*string)(nil)).Return(&db.User{
					ID:             "test-user-123",
					Email:          "test@example.com",
					OrganisationID: stringPtr("org-123"),
				}, nil)

				slowPages := []db.SlowPage{
					{
						URL:                "https://example.com/slow-page",
						Domain:             "example.com",
						Path:               "/slow-page",
						SecondResponseTime: 5000,
						JobID:              "job-123",
						CompletedAt:        "2024-01-01T12:00:00Z",
					},
				}
				mockDB.On("GetSlowPages", "org-123", mock.AnythingOfType("*time.Time"), mock.AnythingOfType("*time.Time")).Return(slowPages, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, "success", response["status"])
				data := response["data"].(map[string]interface{})
				assert.Contains(t, data, "slow_pages")
				assert.Contains(t, data, "count")
				assert.Equal(t, float64(1), data["count"])

				slowPages := data["slow_pages"].([]interface{})
				assert.Len(t, slowPages, 1)
				page := slowPages[0].(map[string]interface{})
				assert.Equal(t, "https://example.com/slow-page", page["url"])
				assert.Equal(t, float64(5000), page["second_response_time"])
			},
		},
		{
			name:        "slow_pages_database_error",
			queryParams: "",
			setupMocks: func(mockDB *MockDBClient, mockJM *MockJobManager) {
				mockDB.On("GetOrCreateUser", "test-user-123", "test@example.com", (*string)(nil)).Return(&db.User{
					ID:             "test-user-123",
					Email:          "test@example.com",
					OrganisationID: stringPtr("org-123"),
				}, nil)

				mockDB.On("GetSlowPages", "org-123", mock.AnythingOfType("*time.Time"), mock.AnythingOfType("*time.Time")).Return(nil, assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:        "slow_pages_no_authentication",
			queryParams: "",
			setupMocks: func(mockDB *MockDBClient, mockJM *MockJobManager) {
				// No setup - will fail authentication
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:        "slow_pages_with_custom_range",
			queryParams: "?range=last30",
			setupMocks: func(mockDB *MockDBClient, mockJM *MockJobManager) {
				mockDB.On("GetOrCreateUser", "test-user-123", "test@example.com", (*string)(nil)).Return(&db.User{
					ID:             "test-user-123",
					Email:          "test@example.com",
					OrganisationID: stringPtr("org-123"),
				}, nil)

				mockDB.On("GetSlowPages", "org-123", mock.AnythingOfType("*time.Time"), mock.AnythingOfType("*time.Time")).Return([]db.SlowPage{}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, "success", response["status"])
				data := response["data"].(map[string]interface{})
				assert.Equal(t, "last30", data["date_range"])
				assert.Equal(t, float64(0), data["count"])
			},
		},
		{
			name:        "slow_pages_user_creation_error",
			queryParams: "",
			setupMocks: func(mockDB *MockDBClient, mockJM *MockJobManager) {
				mockDB.On("GetOrCreateUser", "test-user-123", "test@example.com", (*string)(nil)).Return(nil, assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockDB := &MockDBClient{}
			mockJobsManager := &MockJobManager{}

			tt.setupMocks(mockDB, mockJobsManager)

			handler := NewHandler(mockDB, mockJobsManager)
			rec := httptest.NewRecorder()

			var req *http.Request
			if tt.name == "slow_pages_no_authentication" {
				req = httptest.NewRequest(http.MethodGet, "/v1/dashboard/slow-pages"+tt.queryParams, nil)
			} else {
				req = createAuthenticatedRequest(http.MethodGet, "/v1/dashboard/slow-pages"+tt.queryParams, nil)
			}

			// Execute
			handler.DashboardSlowPages(rec, req)

			// Verify
			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}

			// Verify mocks
			mockDB.AssertExpectations(t)
			mockJobsManager.AssertExpectations(t)
		})
	}
}

func TestDashboardExternalRedirectsIntegration(t *testing.T) {
	tests := []struct {
		name           string
		queryParams    string
		setupMocks     func(*MockDBClient, *MockJobManager)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:        "successful_redirects_default_range",
			queryParams: "",
			setupMocks: func(mockDB *MockDBClient, mockJM *MockJobManager) {
				mockDB.On("GetOrCreateUser", "test-user-123", "test@example.com", (*string)(nil)).Return(&db.User{
					ID:             "test-user-123",
					Email:          "test@example.com",
					OrganisationID: stringPtr("org-123"),
				}, nil)

				redirects := []db.ExternalRedirect{
					{
						URL:         "https://example.com/redirect",
						Domain:      "example.com",
						Path:        "/redirect",
						RedirectURL: "https://external-site.com/target",
						JobID:       "job-123",
						CompletedAt: "2024-01-01T12:00:00Z",
					},
				}
				mockDB.On("GetExternalRedirects", "org-123", mock.AnythingOfType("*time.Time"), mock.AnythingOfType("*time.Time")).Return(redirects, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, "success", response["status"])
				data := response["data"].(map[string]interface{})
				assert.Contains(t, data, "external_redirects")
				assert.Contains(t, data, "count")
				assert.Equal(t, float64(1), data["count"])

				redirects := data["external_redirects"].([]interface{})
				assert.Len(t, redirects, 1)
				redirect := redirects[0].(map[string]interface{})
				assert.Equal(t, "https://example.com/redirect", redirect["url"])
				assert.Equal(t, "https://external-site.com/target", redirect["redirect_url"])
			},
		},
		{
			name:        "redirects_database_error",
			queryParams: "",
			setupMocks: func(mockDB *MockDBClient, mockJM *MockJobManager) {
				mockDB.On("GetOrCreateUser", "test-user-123", "test@example.com", (*string)(nil)).Return(&db.User{
					ID:             "test-user-123",
					Email:          "test@example.com",
					OrganisationID: stringPtr("org-123"),
				}, nil)

				mockDB.On("GetExternalRedirects", "org-123", mock.AnythingOfType("*time.Time"), mock.AnythingOfType("*time.Time")).Return(nil, assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:        "redirects_no_authentication",
			queryParams: "",
			setupMocks: func(mockDB *MockDBClient, mockJM *MockJobManager) {
				// No setup - will fail authentication
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:        "redirects_with_custom_range",
			queryParams: "?range=last7",
			setupMocks: func(mockDB *MockDBClient, mockJM *MockJobManager) {
				mockDB.On("GetOrCreateUser", "test-user-123", "test@example.com", (*string)(nil)).Return(&db.User{
					ID:             "test-user-123",
					Email:          "test@example.com",
					OrganisationID: stringPtr("org-123"),
				}, nil)

				mockDB.On("GetExternalRedirects", "org-123", mock.AnythingOfType("*time.Time"), mock.AnythingOfType("*time.Time")).Return([]db.ExternalRedirect{}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, "success", response["status"])
				data := response["data"].(map[string]interface{})
				assert.Equal(t, "last7", data["date_range"])
				assert.Equal(t, float64(0), data["count"])
			},
		},
		{
			name:        "redirects_user_creation_error",
			queryParams: "",
			setupMocks: func(mockDB *MockDBClient, mockJM *MockJobManager) {
				mockDB.On("GetOrCreateUser", "test-user-123", "test@example.com", (*string)(nil)).Return(nil, assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:        "redirects_empty_organisation",
			queryParams: "",
			setupMocks: func(mockDB *MockDBClient, mockJM *MockJobManager) {
				mockDB.On("GetOrCreateUser", "test-user-123", "test@example.com", (*string)(nil)).Return(&db.User{
					ID:             "test-user-123",
					Email:          "test@example.com",
					OrganisationID: nil, // No organisation
				}, nil)

				mockDB.On("GetExternalRedirects", "", mock.AnythingOfType("*time.Time"), mock.AnythingOfType("*time.Time")).Return([]db.ExternalRedirect{}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, "success", response["status"])
				data := response["data"].(map[string]interface{})
				assert.Equal(t, float64(0), data["count"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockDB := &MockDBClient{}
			mockJobsManager := &MockJobManager{}

			tt.setupMocks(mockDB, mockJobsManager)

			handler := NewHandler(mockDB, mockJobsManager)
			rec := httptest.NewRecorder()

			var req *http.Request
			if tt.name == "redirects_no_authentication" {
				req = httptest.NewRequest(http.MethodGet, "/v1/dashboard/external-redirects"+tt.queryParams, nil)
			} else {
				req = createAuthenticatedRequest(http.MethodGet, "/v1/dashboard/external-redirects"+tt.queryParams, nil)
			}

			// Execute
			handler.DashboardExternalRedirects(rec, req)

			// Verify
			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}

			// Verify mocks
			mockDB.AssertExpectations(t)
			mockJobsManager.AssertExpectations(t)
		})
	}
}
