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

func (m *MockDBClient) ListJobs(organisationID string, limit, offset int, status, dateRange, timezone string) ([]db.JobWithDomain, int, error) {
	args := m.Called(organisationID, limit, offset, status, dateRange, timezone)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]db.JobWithDomain), args.Int(1), args.Error(2)
}

func (m *MockDBClient) ListJobsWithOffset(organisationID string, limit, offset int, status, dateRange string, tzOffsetMinutes int) ([]db.JobWithDomain, int, error) {
	args := m.Called(organisationID, limit, offset, status, dateRange, tzOffsetMinutes)
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

func (m *MockDBClient) ResetDataOnly() error {
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

func (m *MockDBClient) CreateScheduler(ctx context.Context, scheduler *db.Scheduler) error {
	args := m.Called(ctx, scheduler)
	return args.Error(0)
}

func (m *MockDBClient) GetScheduler(ctx context.Context, schedulerID string) (*db.Scheduler, error) {
	args := m.Called(ctx, schedulerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.Scheduler), args.Error(1)
}

func (m *MockDBClient) ListSchedulers(ctx context.Context, organisationID string) ([]*db.Scheduler, error) {
	args := m.Called(ctx, organisationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*db.Scheduler), args.Error(1)
}

func (m *MockDBClient) UpdateScheduler(ctx context.Context, schedulerID string, updates *db.Scheduler) error {
	args := m.Called(ctx, schedulerID, updates)
	return args.Error(0)
}

func (m *MockDBClient) DeleteScheduler(ctx context.Context, schedulerID string) error {
	args := m.Called(ctx, schedulerID)
	return args.Error(0)
}

func (m *MockDBClient) GetSchedulersReadyToRun(ctx context.Context, limit int) ([]*db.Scheduler, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*db.Scheduler), args.Error(1)
}

func (m *MockDBClient) UpdateSchedulerNextRun(ctx context.Context, schedulerID string, nextRun time.Time) error {
	args := m.Called(ctx, schedulerID, nextRun)
	return args.Error(0)
}

func (m *MockDBClient) GetLastJobStartTimeForScheduler(ctx context.Context, schedulerID string) (*time.Time, error) {
	args := m.Called(ctx, schedulerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*time.Time), args.Error(1)
}

func (m *MockDBClient) GetDomainNameByID(ctx context.Context, domainID int) (string, error) {
	args := m.Called(ctx, domainID)
	return args.String(0), args.Error(1)
}

func (m *MockDBClient) GetDomainNames(ctx context.Context, domainIDs []int) (map[int]string, error) {
	args := m.Called(ctx, domainIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[int]string), args.Error(1)
}

func (m *MockDBClient) ListUserOrganisations(userID string) ([]db.UserOrganisation, error) {
	args := m.Called(userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]db.UserOrganisation), args.Error(1)
}

func (m *MockDBClient) ValidateOrganisationMembership(userID, organisationID string) (bool, error) {
	args := m.Called(userID, organisationID)
	return args.Bool(0), args.Error(1)
}

func (m *MockDBClient) SetActiveOrganisation(userID, organisationID string) error {
	args := m.Called(userID, organisationID)
	return args.Error(0)
}

func (m *MockDBClient) GetEffectiveOrganisationID(user *db.User) string {
	args := m.Called(user)
	return args.String(0)
}

func (m *MockDBClient) CreateOrganisation(name string) (*db.Organisation, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.Organisation), args.Error(1)
}

func (m *MockDBClient) AddOrganisationMember(userID, organisationID string) error {
	args := m.Called(userID, organisationID)
	return args.Error(0)
}

// Slack integration mock methods
func (m *MockDBClient) CreateSlackConnection(ctx context.Context, conn *db.SlackConnection) error {
	args := m.Called(ctx, conn)
	return args.Error(0)
}

func (m *MockDBClient) GetSlackConnection(ctx context.Context, connectionID string) (*db.SlackConnection, error) {
	args := m.Called(ctx, connectionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.SlackConnection), args.Error(1)
}

func (m *MockDBClient) ListSlackConnections(ctx context.Context, organisationID string) ([]*db.SlackConnection, error) {
	args := m.Called(ctx, organisationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*db.SlackConnection), args.Error(1)
}

func (m *MockDBClient) DeleteSlackConnection(ctx context.Context, connectionID, organisationID string) error {
	args := m.Called(ctx, connectionID, organisationID)
	return args.Error(0)
}

func (m *MockDBClient) CreateSlackUserLink(ctx context.Context, link *db.SlackUserLink) error {
	args := m.Called(ctx, link)
	return args.Error(0)
}

func (m *MockDBClient) GetSlackUserLink(ctx context.Context, userID, connectionID string) (*db.SlackUserLink, error) {
	args := m.Called(ctx, userID, connectionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.SlackUserLink), args.Error(1)
}

func (m *MockDBClient) UpdateSlackUserLinkNotifications(ctx context.Context, userID, connectionID string, dmNotifications bool) error {
	args := m.Called(ctx, userID, connectionID, dmNotifications)
	return args.Error(0)
}

func (m *MockDBClient) DeleteSlackUserLink(ctx context.Context, userID, connectionID string) error {
	args := m.Called(ctx, userID, connectionID)
	return args.Error(0)
}

func (m *MockDBClient) StoreSlackToken(ctx context.Context, connectionID, token string) error {
	args := m.Called(ctx, connectionID, token)
	return args.Error(0)
}

func (m *MockDBClient) GetSlackToken(ctx context.Context, connectionID string) (string, error) {
	args := m.Called(ctx, connectionID)
	return args.String(0), args.Error(1)
}

func (m *MockDBClient) ListNotifications(ctx context.Context, organisationID string, limit, offset int, unreadOnly bool) ([]*db.Notification, int, error) {
	args := m.Called(ctx, organisationID, limit, offset, unreadOnly)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*db.Notification), args.Int(1), args.Error(2)
}

func (m *MockDBClient) GetUnreadNotificationCount(ctx context.Context, organisationID string) (int, error) {
	args := m.Called(ctx, organisationID)
	return args.Int(0), args.Error(1)
}

func (m *MockDBClient) MarkNotificationRead(ctx context.Context, notificationID, organisationID string) error {
	args := m.Called(ctx, notificationID, organisationID)
	return args.Error(0)
}

func (m *MockDBClient) MarkAllNotificationsRead(ctx context.Context, organisationID string) error {
	args := m.Called(ctx, organisationID)
	return args.Error(0)
}

// Webflow integration mock methods
func (m *MockDBClient) CreateWebflowConnection(ctx context.Context, conn *db.WebflowConnection) error {
	args := m.Called(ctx, conn)
	return args.Error(0)
}

func (m *MockDBClient) GetWebflowConnection(ctx context.Context, connectionID string) (*db.WebflowConnection, error) {
	args := m.Called(ctx, connectionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.WebflowConnection), args.Error(1)
}

func (m *MockDBClient) ListWebflowConnections(ctx context.Context, organisationID string) ([]*db.WebflowConnection, error) {
	args := m.Called(ctx, organisationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*db.WebflowConnection), args.Error(1)
}

func (m *MockDBClient) DeleteWebflowConnection(ctx context.Context, connectionID, organisationID string) error {
	args := m.Called(ctx, connectionID, organisationID)
	return args.Error(0)
}

func (m *MockDBClient) StoreWebflowToken(ctx context.Context, connectionID, token string) error {
	args := m.Called(ctx, connectionID, token)
	return args.Error(0)
}

func (m *MockDBClient) GetWebflowToken(ctx context.Context, connectionID string) (string, error) {
	args := m.Called(ctx, connectionID)
	return args.String(0), args.Error(1)
}

// Google Analytics integration mock methods
func (m *MockDBClient) CreateGoogleConnection(ctx context.Context, conn *db.GoogleAnalyticsConnection) error {
	args := m.Called(ctx, conn)
	return args.Error(0)
}

func (m *MockDBClient) GetGoogleConnection(ctx context.Context, connectionID string) (*db.GoogleAnalyticsConnection, error) {
	args := m.Called(ctx, connectionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.GoogleAnalyticsConnection), args.Error(1)
}

func (m *MockDBClient) ListGoogleConnections(ctx context.Context, organisationID string) ([]*db.GoogleAnalyticsConnection, error) {
	args := m.Called(ctx, organisationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*db.GoogleAnalyticsConnection), args.Error(1)
}

func (m *MockDBClient) DeleteGoogleConnection(ctx context.Context, connectionID, organisationID string) error {
	args := m.Called(ctx, connectionID, organisationID)
	return args.Error(0)
}

func (m *MockDBClient) StoreGoogleToken(ctx context.Context, connectionID, refreshToken string) error {
	args := m.Called(ctx, connectionID, refreshToken)
	return args.Error(0)
}

func (m *MockDBClient) GetGoogleToken(ctx context.Context, connectionID string) (string, error) {
	args := m.Called(ctx, connectionID)
	return args.String(0), args.Error(1)
}

// Platform integration mock methods
func (m *MockDBClient) UpsertPlatformOrgMapping(ctx context.Context, mapping *db.PlatformOrgMapping) error {
	args := m.Called(ctx, mapping)
	return args.Error(0)
}

func (m *MockDBClient) GetPlatformOrgMapping(ctx context.Context, platform, platformID string) (*db.PlatformOrgMapping, error) {
	args := m.Called(ctx, platform, platformID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.PlatformOrgMapping), args.Error(1)
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
