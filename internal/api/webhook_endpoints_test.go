package api

import (
	"bytes"
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
		{
			name: "webhook_wrong_method",
			path: "/v1/webhooks/webflow/test-token",
			requestBody: map[string]interface{}{
				"triggerType": "site_publish",
			},
			setupMocks: func(mockDB *MockDBClient, jm *MockJobManager) {
				// No mocks needed - should fail before DB calls
			},
			expectedStatus: http.StatusMethodNotAllowed,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, float64(405), response["status"])
				assert.Equal(t, "METHOD_NOT_ALLOWED", response["code"])
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

			method := http.MethodPost
			if tt.name == "webhook_wrong_method" {
				method = http.MethodGet // Wrong method for testing
			}

			req := httptest.NewRequest(method, tt.path, bytes.NewBuffer(requestBody))
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

// TestWebflowWebhookEdgeCases tests additional edge cases for comprehensive coverage
func TestWebflowWebhookEdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		requestBody    interface{} // Use interface{} for invalid JSON testing
		setupMocks     func(*MockDBClient, *MockJobManager)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "webhook_non_site_publish_trigger",
			path: "/v1/webhooks/webflow/test-token",
			requestBody: map[string]interface{}{
				"triggerType": "form_submission", // Not site_publish
				"payload": map[string]interface{}{
					"domains": []string{"example.com"},
				},
			},
			setupMocks: func(mockDB *MockDBClient, jm *MockJobManager) {
				user := &db.User{
					ID:             "webhook-user-123",
					Email:          "webhook@example.com",
					OrganisationID: stringPtr("webhook-org-456"),
				}
				mockDB.On("GetUserByWebhookToken", "test-token").Return(user, nil)
				// No JobManager mocks - should be ignored
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, "success", response["status"])
				assert.Contains(t, response["message"].(string), "ignored")
			},
		},
		{
			name: "webhook_job_creation_failure",
			path: "/v1/webhooks/webflow/test-token",
			requestBody: map[string]interface{}{
				"triggerType": "site_publish",
				"payload": map[string]interface{}{
					"domains": []string{"example.com"},
					"publishedBy": map[string]interface{}{
						"displayName": "Test Publisher",
					},
				},
			},
			setupMocks: func(mockDB *MockDBClient, jm *MockJobManager) {
				user := &db.User{
					ID:             "webhook-user-123",
					Email:          "webhook@example.com",
					OrganisationID: stringPtr("webhook-org-456"),
				}
				mockDB.On("GetUserByWebhookToken", "test-token").Return(user, nil)

				// Mock job creation failure
				jm.On("CreateJob", mock.Anything, mock.AnythingOfType("*jobs.JobOptions")).Return(nil, assert.AnError)
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
			var req *http.Request
			if tt.name == "webhook_invalid_json_payload" {
				// Send invalid JSON
				req = httptest.NewRequest(http.MethodPost, tt.path, bytes.NewBufferString("{invalid json}"))
			} else {
				requestBody, err := json.Marshal(tt.requestBody)
				require.NoError(t, err)
				req = httptest.NewRequest(http.MethodPost, tt.path, bytes.NewBuffer(requestBody))
			}
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
