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
				}).AddRow(
					100, 85, 10, 5, // task counts
					"completed", "example.com", // status and domain
					time.Now(), time.Now().Add(-time.Hour), time.Now(), // timestamps
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
				
				data := response["data"].(map[string]interface{})
				assert.Equal(t, "job-123", data["id"])
				assert.Equal(t, "example.com", data["domain"])
				assert.Equal(t, "completed", data["status"])
				assert.Equal(t, float64(100), data["total_tasks"])
				assert.Equal(t, float64(85), data["completed_tasks"])
				assert.Equal(t, float64(10), data["failed_tasks"])
				assert.Equal(t, float64(5), data["skipped_tasks"])
				// Progress should be (85+10)/(100-5) = 95/95 = 100%
				assert.Equal(t, float64(100), data["progress"])
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
			expectedStatus: http.StatusNotFound, // getJob returns 404 for any DB error
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				
				assert.Equal(t, float64(404), response["status"])
				assert.Equal(t, "NOT_FOUND", response["code"])
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
					CreatedAt:      time.Now(),
					CompletedAt:    time.Now(),
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
			
			// Create request with action
			requestBody := map[string]string{"action": tt.action}
			bodyBytes, err := json.Marshal(requestBody)
			require.NoError(t, err)
			
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
			
			// Verify all SQL expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())
			
			// Verify mocks
			mockDB.AssertExpectations(t)
			mockJobsManager.AssertExpectations(t)
		})
	}
}