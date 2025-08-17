package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/auth"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/Harvey-AU/blue-banded-bee/internal/jobs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockJobManager is a mock implementation of jobs.JobManagerInterface
type MockJobManager struct {
	mock.Mock
}

func (m *MockJobManager) CreateJob(ctx context.Context, options *jobs.JobOptions) (*jobs.Job, error) {
	args := m.Called(ctx, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*jobs.Job), args.Error(1)
}

func (m *MockJobManager) StartJob(ctx context.Context, jobID string) error {
	args := m.Called(ctx, jobID)
	return args.Error(0)
}

func (m *MockJobManager) CancelJob(ctx context.Context, jobID string) error {
	args := m.Called(ctx, jobID)
	return args.Error(0)
}

func (m *MockJobManager) GetJobStatus(ctx context.Context, jobID string) (*jobs.Job, error) {
	args := m.Called(ctx, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*jobs.Job), args.Error(1)
}

func (m *MockJobManager) GetJob(ctx context.Context, jobID string) (*jobs.Job, error) {
	args := m.Called(ctx, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*jobs.Job), args.Error(1)
}

func (m *MockJobManager) EnqueueJobURLs(ctx context.Context, jobID string, pages []db.Page, sourceType string, sourceURL string) error {
	args := m.Called(ctx, jobID, pages, sourceType, sourceURL)
	return args.Error(0)
}

func (m *MockJobManager) IsJobComplete(job *jobs.Job) bool {
	args := m.Called(job)
	return args.Bool(0)
}

func (m *MockJobManager) CalculateJobProgress(job *jobs.Job) float64 {
	args := m.Called(job)
	return args.Get(0).(float64)
}

func (m *MockJobManager) ValidateStatusTransition(from, to jobs.JobStatus) error {
	args := m.Called(from, to)
	return args.Error(0)
}

func (m *MockJobManager) UpdateJobStatus(ctx context.Context, jobID string, status jobs.JobStatus) error {
	args := m.Called(ctx, jobID, status)
	return args.Error(0)
}

// MockDBClient is a mock implementation of DBClient interface
type MockDBClient struct {
	mock.Mock
}

func (m *MockDBClient) GetDB() *sql.DB {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*sql.DB)
}

func (m *MockDBClient) GetOrCreateUser(userID, email string, orgID *string) (*db.User, error) {
	args := m.Called(userID, email, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.User), args.Error(1)
}

func (m *MockDBClient) GetUserByWebhookToken(token string) (*db.User, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.User), args.Error(1)
}

func (m *MockDBClient) ListJobs(organisationID string, limit, offset int, status, dateRange string) ([]db.JobWithDomain, int, error) {
	args := m.Called(organisationID, limit, offset, status, dateRange)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]db.JobWithDomain), args.Int(1), args.Error(2)
}

func (m *MockDBClient) GetJobStats(organisationID string, startDate, endDate *time.Time) (*db.JobStats, error) {
	args := m.Called(organisationID, startDate, endDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.JobStats), args.Error(1)
}

func (m *MockDBClient) GetJobActivity(organisationID string, startDate, endDate *time.Time) ([]db.ActivityPoint, error) {
	args := m.Called(organisationID, startDate, endDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]db.ActivityPoint), args.Error(1)
}

func (m *MockDBClient) GetUser(userID string) (*db.User, error) {
	args := m.Called(userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.User), args.Error(1)
}

func (m *MockDBClient) ResetSchema() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDBClient) CreateUser(userID, email string, fullName *string, orgName string) (*db.User, *db.Organisation, error) {
	args := m.Called(userID, email, fullName, orgName)
	var user *db.User
	var org *db.Organisation
	if args.Get(0) != nil {
		user = args.Get(0).(*db.User)
	}
	if args.Get(1) != nil {
		org = args.Get(1).(*db.Organisation)
	}
	return user, org, args.Error(2)
}

func (m *MockDBClient) GetOrganisation(organisationID string) (*db.Organisation, error) {
	args := m.Called(organisationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.Organisation), args.Error(1)
}

// Test helpers
func createTestHandler() (*Handler, *MockDBClient, *MockJobManager) {
	mockDB := new(MockDBClient)
	mockJobsManager := new(MockJobManager)
	
	handler := NewHandler(mockDB, mockJobsManager)
	return handler, mockDB, mockJobsManager
}

func createAuthenticatedRequest(method, url string, body []byte) *http.Request {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, url, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, url, nil)
	}
	
	// Add user context for authentication
	ctx := context.WithValue(req.Context(), auth.UserKey, &auth.UserClaims{
		UserID: "test-user-123",
		Email:  "test@example.com",
	})
	return req.WithContext(ctx)
}

func TestCreateJobIntegration(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    CreateJobRequest
		setupMocks     func(*MockDBClient, *MockJobManager)
		expectedStatus int
		expectedJob    *jobs.Job
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "successful_job_creation_with_defaults",
			requestBody: CreateJobRequest{
				Domain: "example.com",
			},
			setupMocks: func(mockDB *MockDBClient, jm *MockJobManager) {
				orgID := "org-123"
				user := &db.User{
					ID:             "test-user-123",
					Email:          "test@example.com",
					OrganisationID: &orgID,
				}
				mockDB.On("GetOrCreateUser", "test-user-123", "test@example.com", (*string)(nil)).Return(user, nil)
				
				createdJob := &jobs.Job{
					ID:             "job-456",
					Domain:         "example.com",
					Status:         jobs.JobStatusPending,
					TotalTasks:     0,
					CompletedTasks: 0,
					FailedTasks:    0,
					SkippedTasks:   0,
					CreatedAt:      time.Now(),
				}
				
				// Expect CreateJob to be called with correct options
				jm.On("CreateJob", mock.AnythingOfType("*context.valueCtx"), mock.MatchedBy(func(opts *jobs.JobOptions) bool {
					return opts.Domain == "example.com" &&
						opts.UseSitemap == true &&
						opts.FindLinks == true &&
						opts.Concurrency == 5 &&
						opts.MaxPages == 0 &&
						opts.UserID != nil && *opts.UserID == "test-user-123" &&
						opts.OrganisationID != nil && *opts.OrganisationID == "org-123"
				})).Return(createdJob, nil)
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				
				assert.Equal(t, "success", response["status"])
				assert.Equal(t, "Job created successfully", response["message"])
				
				data := response["data"].(map[string]interface{})
				assert.Equal(t, "job-456", data["id"])
				assert.Equal(t, "example.com", data["domain"])
				assert.Equal(t, "pending", data["status"])
				assert.Equal(t, float64(0), data["total_tasks"])
				assert.Equal(t, float64(0), data["progress"])
			},
		},
		{
			name: "successful_job_creation_with_custom_options",
			requestBody: CreateJobRequest{
				Domain:      "custom.com",
				UseSitemap:  boolPtr(false),
				FindLinks:   boolPtr(false),
				Concurrency: intPtr(10),
				MaxPages:    intPtr(100),
			},
			setupMocks: func(mockDB *MockDBClient, jm *MockJobManager) {
				orgID := "org-123"
				user := &db.User{
					ID:             "test-user-123",
					Email:          "test@example.com",
					OrganisationID: &orgID,
				}
				mockDB.On("GetOrCreateUser", "test-user-123", "test@example.com", (*string)(nil)).Return(user, nil)
				
				createdJob := &jobs.Job{
					ID:             "job-789",
					Domain:         "custom.com",
					Status:         jobs.JobStatusPending,
					TotalTasks:     0,
					CompletedTasks: 0,
					FailedTasks:    0,
					SkippedTasks:   0,
					CreatedAt:      time.Now(),
				}
				
				jm.On("CreateJob", mock.AnythingOfType("*context.valueCtx"), mock.MatchedBy(func(opts *jobs.JobOptions) bool {
					return opts.Domain == "custom.com" &&
						opts.UseSitemap == false &&
						opts.FindLinks == false &&
						opts.Concurrency == 10 &&
						opts.MaxPages == 100
				})).Return(createdJob, nil)
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				
				data := response["data"].(map[string]interface{})
				assert.Equal(t, "job-789", data["id"])
				assert.Equal(t, "custom.com", data["domain"])
			},
		},
		{
			name: "job_creation_missing_domain",
			requestBody: CreateJobRequest{
				// Domain is missing
				UseSitemap: boolPtr(true),
			},
			setupMocks: func(mockDB *MockDBClient, jm *MockJobManager) {
				orgID := "org-123"
				user := &db.User{
					ID:             "test-user-123",
					Email:          "test@example.com",
					OrganisationID: &orgID,
				}
				mockDB.On("GetOrCreateUser", "test-user-123", "test@example.com", (*string)(nil)).Return(user, nil)
				// JobManager should not be called
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				
				// Error responses have status as integer, message as string
				assert.Equal(t, float64(400), response["status"])
				assert.Equal(t, "BAD_REQUEST", response["code"])
				assert.Equal(t, "Domain is required", response["message"])
			},
		},
		{
			name: "job_creation_user_not_found",
			requestBody: CreateJobRequest{
				Domain: "test.com",
			},
			setupMocks: func(mockDB *MockDBClient, jm *MockJobManager) {
				mockDB.On("GetOrCreateUser", "test-user-123", "test@example.com", (*string)(nil)).Return(nil, assert.AnError)
				// JobManager should not be called
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
		{
			name: "job_creation_manager_error",
			requestBody: CreateJobRequest{
				Domain: "error.com",
			},
			setupMocks: func(mockDB *MockDBClient, jm *MockJobManager) {
				orgID := "org-123"
				user := &db.User{
					ID:             "test-user-123",
					Email:          "test@example.com",
					OrganisationID: &orgID,
				}
				mockDB.On("GetOrCreateUser", "test-user-123", "test@example.com", (*string)(nil)).Return(user, nil)
				jm.On("CreateJob", mock.AnythingOfType("*context.valueCtx"), mock.AnythingOfType("*jobs.JobOptions")).Return(nil, assert.AnError)
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
			requestBody, err := json.Marshal(tt.requestBody)
			require.NoError(t, err)
			
			req := createAuthenticatedRequest(http.MethodPost, "/v1/jobs", requestBody)
			rec := httptest.NewRecorder()
			
			// Execute
			handler.createJob(rec, req)
			
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

// TODO: TestGetJobIntegration requires sqlmock for complex SQL query mocking
// Will be implemented in next iteration

// TODO: TestUpdateJobIntegration requires sqlmock for job access validation queries
// Will be implemented in next iteration

// TODO: TestCancelJobIntegration requires sqlmock for job access validation
// The cancelJob function performs direct SQL queries to validate job ownership
// Will be implemented with sqlmock in next iteration

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

func TestWebflowWebhookIntegration(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		requestBody    map[string]interface{}
		setupMocks     func(*MockDBClient, *MockJobManager)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "successful_webhook_job_creation",
			path: "/v1/webhooks/webflow/test-webhook-token-123",
			requestBody: map[string]interface{}{
				"triggerType": "site_publish",
				"payload": map[string]interface{}{
					"domains": []string{"example.com", "staging.webflow.io"},
					"publishedBy": map[string]interface{}{
						"displayName": "Test Publisher",
					},
				},
			},
			setupMocks: func(mockDB *MockDBClient, jm *MockJobManager) {
				// Mock successful user lookup by webhook token
				user := &db.User{
					ID:             "webhook-user-123",
					Email:          "webhook@example.com",
					OrganisationID: stringPtr("webhook-org-456"),
				}
				mockDB.On("GetUserByWebhookToken", "test-webhook-token-123").Return(user, nil)
				
				// Mock successful job creation
				createdJob := &jobs.Job{
					ID:             "webhook-job-789",
					Domain:         "example.com",
					Status:         jobs.JobStatusPending,
					TotalTasks:     0,
					CompletedTasks: 0,
					FailedTasks:    0,
					SkippedTasks:   0,
					CreatedAt:      time.Now(),
				}
				jm.On("CreateJob", mock.Anything, mock.MatchedBy(func(opts *jobs.JobOptions) bool {
					return opts.Domain == "example.com" &&
						opts.UserID != nil && *opts.UserID == "webhook-user-123" &&
						opts.SourceType != nil && *opts.SourceType == "webflow_webhook"
				})).Return(createdJob, nil)
				
				// Mock StartJob call that webhook makes after creation
				jm.On("StartJob", mock.Anything, "webhook-job-789").Return(nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				
				// Verify successful response structure
				assert.Equal(t, "success", response["status"])
				assert.Contains(t, response["message"].(string), "Job created successfully")
				
				// The main success is that the webhook processed correctly
				// (we can see from logs: "Successfully created and started job from Webflow webhook")
			},
		},
		{
			name: "webhook_missing_token",
			path: "/v1/webhooks/webflow/", // No token
			requestBody: map[string]interface{}{
				"triggerType": "site_publish",
				"payload": map[string]interface{}{
					"domains": []string{"example.com"},
				},
			},
			setupMocks: func(mockDB *MockDBClient, jm *MockJobManager) {
				// No mocks needed - should fail before DB calls
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				
				assert.Equal(t, float64(400), response["status"])
				assert.Equal(t, "BAD_REQUEST", response["code"])
				assert.Equal(t, "Webhook token required in URL path", response["message"])
			},
		},
		{
			name: "webhook_invalid_user_token",
			path: "/v1/webhooks/webflow/invalid-token",
			requestBody: map[string]interface{}{
				"triggerType": "site_publish",
				"payload": map[string]interface{}{
					"domains": []string{"example.com"},
				},
			},
			setupMocks: func(mockDB *MockDBClient, jm *MockJobManager) {
				mockDB.On("GetUserByWebhookToken", "invalid-token").Return(nil, assert.AnError)
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				
				assert.Equal(t, float64(404), response["status"])
				assert.Equal(t, "NOT_FOUND", response["code"])
				assert.Equal(t, "Invalid webhook token", response["message"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mockDB, mockJobsManager := createTestHandler()
			
			// Setup mocks
			tt.setupMocks(mockDB, mockJobsManager)
			
			// Create request
			requestBody, err := json.Marshal(tt.requestBody)
			require.NoError(t, err)
			
			req := httptest.NewRequest(http.MethodPost, tt.path, bytes.NewBuffer(requestBody))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			
			// Execute
			handler.WebflowWebhook(rec, req)
			
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

// Helper functions
func boolPtr(b bool) *bool {
	return &b
}

func intPtr(i int) *int {
	return &i
}

func stringPtr(s string) *string {
	return &s
}



// Benchmark tests for the new integration tests
func BenchmarkCreateJobIntegration(b *testing.B) {
	handler, mockDB, mockJobsManager := createTestHandler()
	
	// Setup mocks
	orgID := "org-123"
	user := &db.User{
		ID:             "test-user-123",
		Email:          "test@example.com",
		OrganisationID: &orgID,
	}
	mockDB.On("GetOrCreateUser", mock.Anything, mock.Anything, mock.Anything).Return(user, nil)
	
	job := &jobs.Job{
		ID:     "job-123",
		Domain: "example.com",
		Status: jobs.JobStatusPending,
	}
	mockJobsManager.On("CreateJob", mock.Anything, mock.Anything).Return(job, nil)
	
	// Prepare request
	requestBody := CreateJobRequest{Domain: "example.com"}
	body, _ := json.Marshal(requestBody)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := createAuthenticatedRequest(http.MethodPost, "/v1/jobs", body)
		rec := httptest.NewRecorder()
		handler.createJob(rec, req)
	}
}