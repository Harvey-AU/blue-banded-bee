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

func TestDashboardStatsIntegration(t *testing.T) {
	tests := []struct {
		name           string
		queryParams    string
		setupMocks     func(*MockDBClient, *MockJobManager)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:        "successful_stats_default_range",
			queryParams: "",
			setupMocks: func(mockDB *MockDBClient, jm *MockJobManager) {
				orgID := "org-123"
				user := &db.User{
					ID:             "test-user-123",
					Email:          "test@example.com",
					OrganisationID: &orgID,
				}
				mockDB.On("GetOrCreateUser", "test-user-123", "test@example.com", (*string)(nil)).Return(user, nil)
				
				// Mock successful job stats
				stats := &db.JobStats{
					TotalJobs:         10,
					CompletedJobs:     8,
					FailedJobs:        1,
					RunningJobs:       1,
					TotalTasks:        500,
					AvgCompletionTime: 2.5,
				}
				mockDB.On("GetJobStats", "org-123", mock.AnythingOfType("*time.Time"), mock.AnythingOfType("*time.Time")).Return(stats, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				
				assert.Equal(t, "success", response["status"])
				assert.Equal(t, "Dashboard statistics retrieved successfully", response["message"])
				
				data := response["data"].(map[string]interface{})
				assert.Equal(t, float64(10), data["total_jobs"])
				assert.Equal(t, float64(8), data["completed_jobs"])
				assert.Equal(t, float64(1), data["failed_jobs"])
				assert.Equal(t, float64(1), data["running_jobs"])
				assert.Equal(t, float64(500), data["total_tasks"])
				assert.Equal(t, 2.5, data["avg_completion_time"])
			},
		},
		{
			name:        "stats_with_custom_range",
			queryParams: "?range=last30",
			setupMocks: func(mockDB *MockDBClient, jm *MockJobManager) {
				orgID := "org-123"
				user := &db.User{
					ID:             "test-user-123",
					Email:          "test@example.com",
					OrganisationID: &orgID,
				}
				mockDB.On("GetOrCreateUser", "test-user-123", "test@example.com", (*string)(nil)).Return(user, nil)
				
				stats := &db.JobStats{
					TotalJobs:         25,
					CompletedJobs:     20,
					FailedJobs:        3,
					RunningJobs:       2,
					TotalTasks:        1200,
					AvgCompletionTime: 3.2,
				}
				mockDB.On("GetJobStats", "org-123", mock.AnythingOfType("*time.Time"), mock.AnythingOfType("*time.Time")).Return(stats, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				
				data := response["data"].(map[string]interface{})
				assert.Equal(t, float64(25), data["total_jobs"])
				assert.Equal(t, float64(1200), data["total_tasks"])
				assert.Equal(t, 3.2, data["avg_completion_time"])
			},
		},
		{
			name:        "stats_database_error",
			queryParams: "",
			setupMocks: func(mockDB *MockDBClient, jm *MockJobManager) {
				orgID := "org-123"
				user := &db.User{
					ID:             "test-user-123",
					Email:          "test@example.com",
					OrganisationID: &orgID,
				}
				mockDB.On("GetOrCreateUser", "test-user-123", "test@example.com", (*string)(nil)).Return(user, nil)
				mockDB.On("GetJobStats", "org-123", mock.AnythingOfType("*time.Time"), mock.AnythingOfType("*time.Time")).Return(nil, assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				
				assert.Equal(t, float64(500), response["status"])
				assert.Equal(t, "DATABASE_ERROR", response["code"])
			},
		},
		{
			name:        "stats_user_creation_error",
			queryParams: "",
			setupMocks: func(mockDB *MockDBClient, jm *MockJobManager) {
				mockDB.On("GetOrCreateUser", "test-user-123", "test@example.com", (*string)(nil)).Return(nil, assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				
				assert.Equal(t, float64(500), response["status"])
				assert.Equal(t, "INTERNAL_ERROR", response["code"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mockDB, mockJobsManager := createTestHandler()
			
			// Setup mocks
			tt.setupMocks(mockDB, mockJobsManager)
			
			// Create request
			req := createAuthenticatedRequest(http.MethodGet, "/v1/dashboard/stats"+tt.queryParams, nil)
			rec := httptest.NewRecorder()
			
			// Execute
			handler.DashboardStats(rec, req)
			
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

func TestDashboardActivityIntegration(t *testing.T) {
	tests := []struct {
		name           string
		queryParams    string
		setupMocks     func(*MockDBClient, *MockJobManager)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:        "successful_activity_default_range",
			queryParams: "",
			setupMocks: func(mockDB *MockDBClient, jm *MockJobManager) {
				orgID := "org-123"
				user := &db.User{
					ID:             "test-user-123",
					Email:          "test@example.com",
					OrganisationID: &orgID,
				}
				mockDB.On("GetOrCreateUser", "test-user-123", "test@example.com", (*string)(nil)).Return(user, nil)
				
				// Mock successful activity data
				activity := []db.ActivityPoint{
					{
						Timestamp:  "2025-08-17T10:00:00Z",
						JobsCount:  3,
						TasksCount: 150,
					},
					{
						Timestamp:  "2025-08-17T11:00:00Z",
						JobsCount:  2,
						TasksCount: 100,
					},
				}
				mockDB.On("GetJobActivity", "org-123", mock.AnythingOfType("*time.Time"), mock.AnythingOfType("*time.Time")).Return(activity, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				
				assert.Equal(t, "success", response["status"])
				assert.Equal(t, "Dashboard activity retrieved successfully", response["message"])
				
				data := response["data"].(map[string]interface{})
				activity := data["activity"].([]interface{})
				assert.Len(t, activity, 2)
				
				// Check first activity point
				firstPoint := activity[0].(map[string]interface{})
				assert.Equal(t, "2025-08-17T10:00:00Z", firstPoint["timestamp"])
				assert.Equal(t, float64(3), firstPoint["jobs_count"])
				assert.Equal(t, float64(150), firstPoint["tasks_count"])
			},
		},
		{
			name:        "activity_database_error",
			queryParams: "",
			setupMocks: func(mockDB *MockDBClient, jm *MockJobManager) {
				orgID := "org-123"
				user := &db.User{
					ID:             "test-user-123",
					Email:          "test@example.com",
					OrganisationID: &orgID,
				}
				mockDB.On("GetOrCreateUser", "test-user-123", "test@example.com", (*string)(nil)).Return(user, nil)
				mockDB.On("GetJobActivity", "org-123", mock.AnythingOfType("*time.Time"), mock.AnythingOfType("*time.Time")).Return(nil, assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				
				assert.Equal(t, float64(500), response["status"])
				assert.Equal(t, "DATABASE_ERROR", response["code"])
			},
		},
		{
			name:        "activity_no_authentication",
			queryParams: "",
			setupMocks: func(mockDB *MockDBClient, jm *MockJobManager) {
				// No authentication context provided
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				
				assert.Equal(t, float64(401), response["status"])
				assert.Equal(t, "UNAUTHORISED", response["code"])
				assert.Equal(t, "Authentication required", response["message"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mockDB, mockJobsManager := createTestHandler()
			
			// Setup mocks
			tt.setupMocks(mockDB, mockJobsManager)
			
			// Create request (authenticated or not based on test)
			var req *http.Request
			if tt.name == "activity_no_authentication" {
				req = httptest.NewRequest(http.MethodGet, "/v1/dashboard/activity"+tt.queryParams, nil)
			} else {
				req = createAuthenticatedRequest(http.MethodGet, "/v1/dashboard/activity"+tt.queryParams, nil)
			}
			rec := httptest.NewRecorder()
			
			// Execute
			handler.DashboardActivity(rec, req)
			
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