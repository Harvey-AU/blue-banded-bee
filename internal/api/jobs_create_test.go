package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/Harvey-AU/blue-banded-bee/internal/jobs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

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
				mockDB.On("GetEffectiveOrganisationID", mock.AnythingOfType("*db.User")).Return(orgID)

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
						opts.Concurrency == 20 && // Default concurrency
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
				mockDB.On("GetEffectiveOrganisationID", mock.AnythingOfType("*db.User")).Return(orgID)

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
				mockDB.On("GetEffectiveOrganisationID", mock.AnythingOfType("*db.User")).Return(orgID)
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
				mockDB.On("GetEffectiveOrganisationID", mock.AnythingOfType("*db.User")).Return(orgID)
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
