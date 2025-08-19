package api

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/auth"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/Harvey-AU/blue-banded-bee/internal/jobs"
	"github.com/stretchr/testify/mock"
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

func (m *MockDBClient) GetSlowPages(organisationID string, startDate, endDate *time.Time) ([]db.SlowPage, error) {
	args := m.Called(organisationID, startDate, endDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]db.SlowPage), args.Error(1)
}

func (m *MockDBClient) GetExternalRedirects(organisationID string, startDate, endDate *time.Time) ([]db.ExternalRedirect, error) {
	args := m.Called(organisationID, startDate, endDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]db.ExternalRedirect), args.Error(1)
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

// Helper functions for pointer creation
func boolPtr(b bool) *bool {
	return &b
}

func intPtr(i int) *int {
	return &i
}

func stringPtr(s string) *string {
	return &s
}