package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Harvey-AU/blue-banded-bee/internal/auth"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/Harvey-AU/blue-banded-bee/internal/jobs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestGetJobIntegration tests the GET /v1/jobs/:id endpoint with sqlmock
func TestGetJobIntegration(t *testing.T) {
	tests := []struct {
		name           string
		jobID          string
		userID         string
		orgID          string
		setupSQL       func(sqlmock.Sqlmock)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:   "successful_job_retrieval",
			jobID:  "job-123",
			userID: "user-456",
			orgID:  "org-789",
			setupSQL: func(mock sqlmock.Sqlmock) {
				// Mock the job query that getJob performs
				rows := sqlmock.NewRows([]string{
					"total_tasks", "completed_tasks", "failed_tasks", "skipped_tasks",
					"status", "domain", "created_at", "started_at", "completed_at",
					"duration_seconds", "avg_time_per_task_seconds", "stats",
				}).AddRow(
					100, 85, 10, 5, // task counts
					"completed", "example.com", // status and domain
					time.Now(), time.Now().Add(-time.Hour), time.Now(), // timestamps
					3600, 42.35, nil, // duration_seconds, avg_time_per_task_seconds, stats (nil for now)
				)

				mock.ExpectQuery(`SELECT j\.total_tasks, j\.completed_tasks, j\.failed_tasks, j\.skipped_tasks, j\.status`).
					WithArgs("job-123", "org-789").
					WillReturnRows(rows)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, "success", response["status"])
				assert.Equal(t, "Job retrieved successfully", response["message"])

				// Safely assert data exists before type assertion
				require.NotNil(t, response["data"], "Response data should not be nil")
				data, ok := response["data"].(map[string]interface{})
				require.True(t, ok, "Response data should be a map")

				assert.Equal(t, "job-123", data["id"])
				assert.Equal(t, "example.com", data["domain"])
				assert.Equal(t, "completed", data["status"])
				assert.Equal(t, float64(100), data["total_tasks"])
				assert.Equal(t, float64(85), data["completed_tasks"])
				assert.Equal(t, float64(10), data["failed_tasks"])
				assert.Equal(t, float64(5), data["skipped_tasks"])
				// Progress should be (85+10)/(100-5) = 95/95 = 100%
				assert.Equal(t, float64(100), data["progress"])

				// Check new fields exist
				assert.NotNil(t, data["duration_seconds"])
				assert.NotNil(t, data["avg_time_per_task_seconds"])
			},
		},
		{
			name:   "job_not_found",
			jobID:  "nonexistent-job",
			userID: "user-456",
			orgID:  "org-789",
			setupSQL: func(mock sqlmock.Sqlmock) {
				// Mock query returning no rows (job not found)
				mock.ExpectQuery(`SELECT j\.total_tasks, j\.completed_tasks`).
					WithArgs("nonexistent-job", "org-789").
					WillReturnError(sql.ErrNoRows)
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, float64(404), response["status"])
				assert.Equal(t, "NOT_FOUND", response["code"])
				assert.Equal(t, "Job not found", response["message"])
			},
		},
		{
			name:   "job_wrong_organisation",
			jobID:  "job-123",
			userID: "user-456",
			orgID:  "wrong-org",
			setupSQL: func(mock sqlmock.Sqlmock) {
				// Mock query returning no rows (different org)
				mock.ExpectQuery(`SELECT j\.total_tasks, j\.completed_tasks`).
					WithArgs("job-123", "wrong-org").
					WillReturnError(sql.ErrNoRows)
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, float64(404), response["status"])
				assert.Equal(t, "NOT_FOUND", response["code"])
			},
		},
		{
			name:   "database_error",
			jobID:  "job-123",
			userID: "user-456",
			orgID:  "org-789",
			setupSQL: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT j\.total_tasks, j\.completed_tasks`).
					WithArgs("job-123", "org-789").
					WillReturnError(assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, float64(http.StatusInternalServerError), response["status"])
				assert.Equal(t, "INTERNAL_ERROR", response["code"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock database
			mockSQL, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer mockSQL.Close()

			// Setup SQL expectations
			tt.setupSQL(mock)

			// Create mock DBClient that returns the mock SQL DB
			mockDB := new(MockDBClient)
			mockJobsManager := new(MockJobManager)

			// Mock GetOrCreateUser for authentication
			user := &db.User{
				ID:             tt.userID,
				Email:          "test@example.com",
				OrganisationID: &tt.orgID,
			}
			mockDB.On("GetOrCreateUser", tt.userID, "test@example.com", (*string)(nil)).Return(user, nil)

			// Mock GetDB to return our sqlmock instance
			mockDB.On("GetDB").Return(mockSQL)

			// Create handler
			handler := NewHandler(mockDB, mockJobsManager)

			// Create authenticated request
			req := httptest.NewRequest(http.MethodGet, "/v1/jobs/"+tt.jobID, nil)
			ctx := context.WithValue(req.Context(), auth.UserKey, &auth.UserClaims{
				UserID: tt.userID,
				Email:  "test@example.com",
			})
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()

			// Execute
			handler.getJob(rec, req, tt.jobID)

			// Verify
			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}

			// Verify all SQL expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())

			// Verify mocks
			mockDB.AssertExpectations(t)
			mockJobsManager.AssertExpectations(t)
		})
	}
}

// TestUpdateJobIntegration tests the PUT /v1/jobs/:id endpoint with sqlmock
func TestUpdateJobIntegration(t *testing.T) {
	tests := []struct {
		name           string
		jobID          string
		userID         string
		orgID          string
		action         string
		setupSQL       func(sqlmock.Sqlmock)
		setupMocks     func(*MockJobManager)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:   "successful_job_start",
			jobID:  "job-123",
			userID: "user-456",
			orgID:  "org-789",
			action: "start",
			setupSQL: func(mock sqlmock.Sqlmock) {
				// Mock job access validation query
				rows := sqlmock.NewRows([]string{"organisation_id"}).AddRow("org-789")
				mock.ExpectQuery(`SELECT organisation_id FROM jobs WHERE id = \$1`).
					WithArgs("job-123").
					WillReturnRows(rows)
			},
			setupMocks: func(jm *MockJobManager) {
				// Mock successful start
				jm.On("StartJob", mock.AnythingOfType("*context.valueCtx"), "job-123").Return(nil)

				// Mock GetJobStatus for response
				job := &jobs.Job{
					ID:             "job-123",
					Domain:         "example.com",
					Status:         jobs.JobStatusRunning,
					TotalTasks:     10,
					CompletedTasks: 0,
					FailedTasks:    0,
					SkippedTasks:   0,
					Progress:       0.0,
					CreatedAt:      time.Now(),
				}
				jm.On("GetJobStatus", mock.AnythingOfType("*context.valueCtx"), "job-123").Return(job, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, "success", response["status"])
				assert.Contains(t, response["message"].(string), "started successfully")

				data := response["data"].(map[string]interface{})
				assert.Equal(t, "job-123", data["id"])
				assert.Equal(t, "example.com", data["domain"])
				assert.Equal(t, "running", data["status"])
			},
		},
		{
			name:   "successful_job_cancel",
			jobID:  "job-456",
			userID: "user-789",
			orgID:  "org-123",
			action: "cancel",
			setupSQL: func(mock sqlmock.Sqlmock) {
				// Mock job access validation query
				rows := sqlmock.NewRows([]string{"organisation_id"}).AddRow("org-123")
				mock.ExpectQuery(`SELECT organisation_id FROM jobs WHERE id = \$1`).
					WithArgs("job-456").
					WillReturnRows(rows)
			},
			setupMocks: func(jm *MockJobManager) {
				// Mock successful cancel
				jm.On("CancelJob", mock.AnythingOfType("*context.valueCtx"), "job-456").Return(nil)

				// Mock GetJobStatus for response
				job := &jobs.Job{
					ID:             "job-456",
					Domain:         "test.com",
					Status:         jobs.JobStatusCancelled,
					TotalTasks:     5,
					CompletedTasks: 2,
					FailedTasks:    0,
					SkippedTasks:   0,
					Progress:       40.0,
					CreatedAt:      time.Now().UTC(),
					CompletedAt:    time.Now().UTC(),
				}
				jm.On("GetJobStatus", mock.AnythingOfType("*context.valueCtx"), "job-456").Return(job, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, "success", response["status"])
				assert.Contains(t, response["message"].(string), "canceled successfully")

				data := response["data"].(map[string]interface{})
				assert.Equal(t, "job-456", data["id"])
				assert.Equal(t, "cancelled", data["status"])
			},
		},
		{
			name:   "job_access_denied_wrong_org",
			jobID:  "job-123",
			userID: "user-456",
			orgID:  "user-org-789", // User's org
			action: "start",
			setupSQL: func(mock sqlmock.Sqlmock) {
				// Mock job access validation returning different org
				rows := sqlmock.NewRows([]string{"organisation_id"}).AddRow("job-org-different")
				mock.ExpectQuery(`SELECT organisation_id FROM jobs WHERE id = \$1`).
					WithArgs("job-123").
					WillReturnRows(rows)
			},
			setupMocks: func(jm *MockJobManager) {
				// JobManager methods should not be called
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, float64(401), response["status"])
				assert.Equal(t, "UNAUTHORISED", response["code"])
				assert.Equal(t, "Job access denied", response["message"])
			},
		},
		{
			name:   "job_not_found",
			jobID:  "nonexistent-job",
			userID: "user-456",
			orgID:  "org-789",
			action: "start",
			setupSQL: func(mock sqlmock.Sqlmock) {
				// Mock job access validation returning no rows
				mock.ExpectQuery(`SELECT organisation_id FROM jobs WHERE id = \$1`).
					WithArgs("nonexistent-job").
					WillReturnError(sql.ErrNoRows)
			},
			setupMocks: func(jm *MockJobManager) {
				// JobManager methods should not be called
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, float64(404), response["status"])
				assert.Equal(t, "NOT_FOUND", response["code"])
				assert.Equal(t, "Job not found", response["message"])
			},
		},
		{
			name:   "invalid_action",
			jobID:  "job-123",
			userID: "user-456",
			orgID:  "org-789",
			action: "invalid",
			setupSQL: func(mock sqlmock.Sqlmock) {
				// Mock job access validation (should pass)
				rows := sqlmock.NewRows([]string{"organisation_id"}).AddRow("org-789")
				mock.ExpectQuery(`SELECT organisation_id FROM jobs WHERE id = \$1`).
					WithArgs("job-123").
					WillReturnRows(rows)
			},
			setupMocks: func(jm *MockJobManager) {
				// JobManager methods should not be called for invalid action
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, float64(400), response["status"])
				assert.Equal(t, "BAD_REQUEST", response["code"])
				assert.Contains(t, response["message"].(string), "Invalid action")
			},
		},
		{
			name:   "invalid_json_payload",
			jobID:  "job-123",
			userID: "user-456",
			orgID:  "org-789",
			action: "invalid-json",
			setupSQL: func(mock sqlmock.Sqlmock) {
				// No SQL expectations - should fail before DB access
			},
			setupMocks: func(jm *MockJobManager) {
				// No JobManager expectations - should fail before JobManager calls
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, float64(400), response["status"])
				assert.Equal(t, "BAD_REQUEST", response["code"])
				assert.Equal(t, "Invalid JSON request body", response["message"])
			},
		},
		{
			name:   "start_job_manager_failure",
			jobID:  "job-123",
			userID: "user-456",
			orgID:  "org-789",
			action: "start",
			setupSQL: func(mock sqlmock.Sqlmock) {
				// Mock job access validation (should pass)
				rows := sqlmock.NewRows([]string{"organisation_id"}).AddRow("org-789")
				mock.ExpectQuery(`SELECT organisation_id FROM jobs WHERE id = \$1`).
					WithArgs("job-123").
					WillReturnRows(rows)
			},
			setupMocks: func(jm *MockJobManager) {
				// Mock StartJob failure
				jm.On("StartJob", mock.AnythingOfType("*context.valueCtx"), "job-123").Return(assert.AnError)
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
			name:   "get_status_failure_after_success",
			jobID:  "job-123",
			userID: "user-456",
			orgID:  "org-789",
			action: "start",
			setupSQL: func(mock sqlmock.Sqlmock) {
				// Mock job access validation (should pass)
				rows := sqlmock.NewRows([]string{"organisation_id"}).AddRow("org-789")
				mock.ExpectQuery(`SELECT organisation_id FROM jobs WHERE id = \$1`).
					WithArgs("job-123").
					WillReturnRows(rows)
			},
			setupMocks: func(jm *MockJobManager) {
				// Mock successful start but failed status retrieval
				jm.On("StartJob", mock.AnythingOfType("*context.valueCtx"), "job-123").Return(nil)
				jm.On("GetJobStatus", mock.AnythingOfType("*context.valueCtx"), "job-123").Return(nil, assert.AnError)
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
			// Create mock database
			mockSQL, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer mockSQL.Close()

			// Setup SQL expectations
			tt.setupSQL(mock)

			// Create mocks
			mockDB := new(MockDBClient)
			mockJobsManager := new(MockJobManager)

			// Setup JobManager mocks
			tt.setupMocks(mockJobsManager)

			// Mock authentication for all tests (auth happens before JSON parsing)
			user := &db.User{
				ID:             tt.userID,
				Email:          "test@example.com",
				OrganisationID: &tt.orgID,
			}
			mockDB.On("GetOrCreateUser", tt.userID, "test@example.com", (*string)(nil)).Return(user, nil)

			// Mock GetDB only for tests that reach SQL queries
			if tt.name != "invalid_json_payload" {
				mockDB.On("GetDB").Return(mockSQL)
			}

			// Create handler
			handler := NewHandler(mockDB, mockJobsManager)

			// Create request with action
			var bodyBytes []byte
			if tt.name == "invalid_json_payload" {
				// Send invalid JSON
				bodyBytes = []byte("{invalid json}")
			} else {
				requestBody := map[string]string{"action": tt.action}
				bodyBytes, err = json.Marshal(requestBody)
				require.NoError(t, err)
			}

			req := createAuthenticatedRequest(http.MethodPut, "/v1/jobs/"+tt.jobID, bodyBytes)
			// Override user context for this specific test
			ctx := context.WithValue(req.Context(), auth.UserKey, &auth.UserClaims{
				UserID: tt.userID,
				Email:  "test@example.com",
			})
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()

			// Execute
			handler.updateJob(rec, req, tt.jobID)

			// Verify
			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}

			// Verify all SQL expectations were met (if any were set)
			if tt.name != "invalid_json_payload" {
				assert.NoError(t, mock.ExpectationsWereMet())
			}

			// Verify mocks
			mockDB.AssertExpectations(t)
			mockJobsManager.AssertExpectations(t)
		})
	}
}

// TestCancelJobIntegration tests the DELETE /v1/jobs/:id endpoint with sqlmock
func TestCancelJobIntegration(t *testing.T) {
	tests := []struct {
		name           string
		jobID          string
		userID         string
		orgID          string
		setupSQL       func(sqlmock.Sqlmock)
		setupMocks     func(*MockJobManager)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:   "successful_job_cancellation",
			jobID:  "job-123",
			userID: "user-456",
			orgID:  "org-789",
			setupSQL: func(mock sqlmock.Sqlmock) {
				// Mock job access validation query
				rows := sqlmock.NewRows([]string{"organisation_id"}).AddRow("org-789")
				mock.ExpectQuery(`SELECT organisation_id FROM jobs WHERE id = \$1`).
					WithArgs("job-123").
					WillReturnRows(rows)
			},
			setupMocks: func(jm *MockJobManager) {
				// Mock successful cancellation
				jm.On("CancelJob", mock.AnythingOfType("*context.valueCtx"), "job-123").Return(nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, "success", response["status"])
				assert.Equal(t, "Job cancelled successfully", response["message"])

				data := response["data"].(map[string]interface{})
				assert.Equal(t, "job-123", data["id"])
				assert.Equal(t, "cancelled", data["status"])
			},
		},
		{
			name:   "cancel_user_creation_error",
			jobID:  "job-123",
			userID: "user-456",
			orgID:  "org-789",
			setupSQL: func(mock sqlmock.Sqlmock) {
				// No SQL expectations - should fail before DB access
			},
			setupMocks: func(jm *MockJobManager) {
				// No JobManager expectations - should fail before JobManager calls
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
			name:   "cancel_manager_error",
			jobID:  "job-123",
			userID: "user-456",
			orgID:  "org-789",
			setupSQL: func(mock sqlmock.Sqlmock) {
				// Mock job access validation (should pass)
				rows := sqlmock.NewRows([]string{"organisation_id"}).AddRow("org-789")
				mock.ExpectQuery(`SELECT organisation_id FROM jobs WHERE id = \$1`).
					WithArgs("job-123").
					WillReturnRows(rows)
			},
			setupMocks: func(jm *MockJobManager) {
				// Mock CancelJob failure
				jm.On("CancelJob", mock.AnythingOfType("*context.valueCtx"), "job-123").Return(assert.AnError)
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
			// Create mock database
			mockSQL, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer mockSQL.Close()

			// Setup SQL expectations
			tt.setupSQL(mock)

			// Create mocks
			mockDB := new(MockDBClient)
			mockJobsManager := new(MockJobManager)

			// Setup JobManager mocks
			tt.setupMocks(mockJobsManager)

			// Mock GetOrCreateUser based on test case
			if tt.name == "cancel_user_creation_error" {
				mockDB.On("GetOrCreateUser", tt.userID, "test@example.com", (*string)(nil)).Return(nil, assert.AnError)
			} else {
				user := &db.User{
					ID:             tt.userID,
					Email:          "test@example.com",
					OrganisationID: &tt.orgID,
				}
				mockDB.On("GetOrCreateUser", tt.userID, "test@example.com", (*string)(nil)).Return(user, nil)
				mockDB.On("GetDB").Return(mockSQL)
			}

			// Create handler
			handler := NewHandler(mockDB, mockJobsManager)

			// Create authenticated request
			req := httptest.NewRequest(http.MethodDelete, "/v1/jobs/"+tt.jobID, nil)
			ctx := context.WithValue(req.Context(), auth.UserKey, &auth.UserClaims{
				UserID: tt.userID,
				Email:  "test@example.com",
			})
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()

			// Execute
			handler.cancelJob(rec, req, tt.jobID)

			// Verify
			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}

			// Verify all SQL expectations were met (only for tests that access DB)
			if tt.name != "cancel_user_creation_error" {
				assert.NoError(t, mock.ExpectationsWereMet())
			}

			// Verify mocks
			mockDB.AssertExpectations(t)
			mockJobsManager.AssertExpectations(t)
		})
	}
}

// TestGetJobTasksIntegration tests the GET /v1/jobs/:id/tasks endpoint with sqlmock
func TestGetJobTasksIntegration(t *testing.T) {
	tests := []struct {
		name           string
		jobID          string
		userID         string
		orgID          string
		queryParams    string
		setupSQL       func(sqlmock.Sqlmock)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:        "successful_tasks_retrieval",
			jobID:       "job-123",
			userID:      "user-456",
			orgID:       "org-789",
			queryParams: "",
			setupSQL: func(mock sqlmock.Sqlmock) {
				// Mock job access validation query (from validateJobAccess)
				orgRows := sqlmock.NewRows([]string{"organisation_id"}).AddRow("org-789")
				mock.ExpectQuery(`SELECT organisation_id FROM jobs WHERE id = \$1`).
					WithArgs("job-123").
					WillReturnRows(orgRows)

				// Mock task count query
				countRows := sqlmock.NewRows([]string{"count"}).AddRow(2)
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM tasks t WHERE t\.job_id = \$1`).
					WithArgs("job-123").
					WillReturnRows(countRows)

				// Mock tasks select query
				taskRows := sqlmock.NewRows([]string{
					"id", "job_id", "path", "domain", "status", "status_code", "response_time",
					"cache_status", "second_response_time", "second_cache_status", "content_type",
					"error", "source_type", "source_url", "created_at", "started_at", "completed_at", "retry_count",
				}).AddRow(
					"task-1", "job-123", "/page1", "example.com", "completed", 200, 150,
					"HIT", 120, "HIT", "text/html", nil, "sitemap", nil,
					time.Now(), time.Now(), time.Now(), 0,
				).AddRow(
					"task-2", "job-123", "/page2", "example.com", "failed", 404, 300,
					"MISS", nil, nil, "text/html", "Not found", "link", "https://example.com/page1",
					time.Now(), time.Now(), time.Now(), 2,
				)

				mock.ExpectQuery(`SELECT t\.id, t\.job_id, p\.path, d\.name as domain, t\.status`).
					WithArgs("job-123", 50, 0).
					WillReturnRows(taskRows)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, "success", response["status"])
				assert.Equal(t, "Tasks retrieved successfully", response["message"])

				data := response["data"].(map[string]interface{})
				tasks := data["tasks"].([]interface{})
				assert.Len(t, tasks, 2)

				// Check first task
				task1 := tasks[0].(map[string]interface{})
				assert.Equal(t, "task-1", task1["id"])
				assert.Equal(t, "/page1", task1["path"])
				assert.Equal(t, "https://example.com/page1", task1["url"])
				assert.Equal(t, "completed", task1["status"])
				assert.Equal(t, float64(200), task1["status_code"])

				// Check pagination
				pagination := data["pagination"].(map[string]interface{})
				assert.Equal(t, float64(50), pagination["limit"])
				assert.Equal(t, float64(0), pagination["offset"])
				assert.Equal(t, float64(2), pagination["total"])
				assert.Equal(t, false, pagination["has_next"])
			},
		},
		{
			name:        "tasks_with_pagination",
			jobID:       "job-456",
			userID:      "user-789",
			orgID:       "org-123",
			queryParams: "?limit=10&offset=20",
			setupSQL: func(mock sqlmock.Sqlmock) {
				// Mock job access validation
				orgRows := sqlmock.NewRows([]string{"organisation_id"}).AddRow("org-123")
				mock.ExpectQuery(`SELECT organisation_id FROM jobs WHERE id = \$1`).
					WithArgs("job-456").
					WillReturnRows(orgRows)

				// Mock task count query
				countRows := sqlmock.NewRows([]string{"count"}).AddRow(50)
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM tasks t WHERE t\.job_id = \$1`).
					WithArgs("job-456").
					WillReturnRows(countRows)

				// Mock tasks select with pagination
				taskRows := sqlmock.NewRows([]string{
					"id", "job_id", "path", "domain", "status", "status_code", "response_time",
					"cache_status", "second_response_time", "second_cache_status", "content_type",
					"error", "source_type", "source_url", "created_at", "started_at", "completed_at", "retry_count",
				}).AddRow(
					"task-21", "job-456", "/page21", "test.com", "completed", 200, 180,
					"HIT", 160, "HIT", "text/html", nil, "sitemap", nil,
					time.Now(), time.Now(), time.Now(), 0,
				)

				mock.ExpectQuery(`SELECT t\.id, t\.job_id, p\.path, d\.name as domain, t\.status`).
					WithArgs("job-456", 10, 20).
					WillReturnRows(taskRows)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)

				data := response["data"].(map[string]interface{})
				pagination := data["pagination"].(map[string]interface{})

				assert.Equal(t, float64(10), pagination["limit"])
				assert.Equal(t, float64(20), pagination["offset"])
				assert.Equal(t, float64(50), pagination["total"])
				assert.Equal(t, true, pagination["has_next"]) // 20 + 10 = 30 < 50
				assert.Equal(t, true, pagination["has_prev"]) // offset > 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock database
			mockSQL, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer mockSQL.Close()

			// Setup SQL expectations
			tt.setupSQL(mock)

			// Create mocks
			mockDB := new(MockDBClient)
			mockJobsManager := new(MockJobManager)

			// Mock GetOrCreateUser for authentication
			user := &db.User{
				ID:             tt.userID,
				Email:          "test@example.com",
				OrganisationID: &tt.orgID,
			}
			mockDB.On("GetOrCreateUser", tt.userID, "test@example.com", (*string)(nil)).Return(user, nil)

			// Mock GetDB to return our sqlmock instance
			mockDB.On("GetDB").Return(mockSQL)

			// Create handler
			handler := NewHandler(mockDB, mockJobsManager)

			// Create authenticated request
			req := httptest.NewRequest(http.MethodGet, "/v1/jobs/"+tt.jobID+"/tasks"+tt.queryParams, nil)
			ctx := context.WithValue(req.Context(), auth.UserKey, &auth.UserClaims{
				UserID: tt.userID,
				Email:  "test@example.com",
			})
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()

			// Execute
			handler.getJobTasks(rec, req, tt.jobID)

			// Verify
			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}

			// Verify all SQL expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())

			// Verify mocks
			mockDB.AssertExpectations(t)
			mockJobsManager.AssertExpectations(t)
		})
	}
}
