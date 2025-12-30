package mocks

import (
	"context"
	"database/sql"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/stretchr/testify/mock"
)

// MockDB is a mock implementation of the database interface
type MockDB struct {
	mock.Mock
}

// GetDB mocks the GetDB method to return underlying *sql.DB
func (m *MockDB) GetDB() *sql.DB {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*sql.DB)
}

// Close mocks the Close method for database connection cleanup
func (m *MockDB) Close() error {
	args := m.Called()
	return args.Error(0)
}

// GetConfig mocks the GetConfig method to return database configuration
func (m *MockDB) GetConfig() *db.Config {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*db.Config)
}

// RecalculateJobStats mocks the RecalculateJobStats method
func (m *MockDB) RecalculateJobStats(ctx context.Context, jobID string) error {
	args := m.Called(ctx, jobID)
	return args.Error(0)
}

// ResetSchema mocks the ResetSchema method for testing database resets
func (m *MockDB) ResetSchema() error {
	args := m.Called()
	return args.Error(0)
}

// ResetDataOnly mocks the ResetDataOnly method for testing data-only resets
func (m *MockDB) ResetDataOnly() error {
	args := m.Called()
	return args.Error(0)
}

// GetOrCreateUser mocks the GetOrCreateUser method
func (m *MockDB) GetOrCreateUser(userID, email string, orgID *string) (*db.User, error) {
	args := m.Called(userID, email, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.User), args.Error(1)
}

// GetUser mocks retrieving a user by ID
func (m *MockDB) GetUser(userID string) (*db.User, error) {
	args := m.Called(userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.User), args.Error(1)
}

// CreateUser mocks creating a user and organisation
func (m *MockDB) CreateUser(userID, email string, fullName *string, orgName string) (*db.User, *db.Organisation, error) {
	args := m.Called(userID, email, fullName, orgName)
	var user *db.User
	var org *db.Organisation
	if u := args.Get(0); u != nil {
		user = u.(*db.User)
	}
	if o := args.Get(1); o != nil {
		org = o.(*db.Organisation)
	}
	return user, org, args.Error(2)
}

// GetOrganisation mocks retrieving an organisation by ID
func (m *MockDB) GetOrganisation(organisationID string) (*db.Organisation, error) {
	args := m.Called(organisationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.Organisation), args.Error(1)
}

// ListJobs mocks listing jobs with pagination and filters
func (m *MockDB) ListJobs(organisationID string, limit, offset int, status, dateRange, timezone string) ([]db.JobWithDomain, int, error) {
	args := m.Called(organisationID, limit, offset, status, dateRange, timezone)
	var jobs []db.JobWithDomain
	if v := args.Get(0); v != nil {
		jobs = v.([]db.JobWithDomain)
	}
	total := 0
	if v := args.Get(1); v != nil {
		total = v.(int)
	}
	return jobs, total, args.Error(2)
}

func (m *MockDB) ListJobsWithOffset(organisationID string, limit, offset int, status, dateRange string, tzOffsetMinutes int) ([]db.JobWithDomain, int, error) {
	args := m.Called(organisationID, limit, offset, status, dateRange, tzOffsetMinutes)
	var jobs []db.JobWithDomain
	if v := args.Get(0); v != nil {
		jobs = v.([]db.JobWithDomain)
	}
	total := 0
	if v := args.Get(1); v != nil {
		total = v.(int)
	}
	return jobs, total, args.Error(2)
}

// GetJobStats mocks the GetJobStats method
func (m *MockDB) GetJobStats(organisationID string, startDate, endDate *time.Time) (*db.JobStats, error) {
	args := m.Called(organisationID, startDate, endDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.JobStats), args.Error(1)
}

// GetJobActivity mocks the GetJobActivity method
func (m *MockDB) GetJobActivity(organisationID string, startDate, endDate *time.Time) ([]db.ActivityPoint, error) {
	args := m.Called(organisationID, startDate, endDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]db.ActivityPoint), args.Error(1)
}

// GetSlowPages mocks the GetSlowPages method
func (m *MockDB) GetSlowPages(organisationID string, startDate, endDate *time.Time) ([]db.SlowPage, error) {
	args := m.Called(organisationID, startDate, endDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]db.SlowPage), args.Error(1)
}

// GetExternalRedirects mocks the GetExternalRedirects method
func (m *MockDB) GetExternalRedirects(organisationID string, startDate, endDate *time.Time) ([]db.ExternalRedirect, error) {
	args := m.Called(organisationID, startDate, endDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]db.ExternalRedirect), args.Error(1)
}

// GetUserByWebhookToken mocks the GetUserByWebhookToken method
func (m *MockDB) GetUserByWebhookToken(token string) (*db.User, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.User), args.Error(1)
}

// CreateScheduler mocks the CreateScheduler method
func (m *MockDB) CreateScheduler(ctx context.Context, scheduler *db.Scheduler) error {
	args := m.Called(ctx, scheduler)
	return args.Error(0)
}

// GetScheduler mocks the GetScheduler method
func (m *MockDB) GetScheduler(ctx context.Context, schedulerID string) (*db.Scheduler, error) {
	args := m.Called(ctx, schedulerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.Scheduler), args.Error(1)
}

// ListSchedulers mocks the ListSchedulers method
func (m *MockDB) ListSchedulers(ctx context.Context, organisationID string) ([]*db.Scheduler, error) {
	args := m.Called(ctx, organisationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*db.Scheduler), args.Error(1)
}

// UpdateScheduler mocks the UpdateScheduler method
func (m *MockDB) UpdateScheduler(ctx context.Context, schedulerID string, updates *db.Scheduler) error {
	args := m.Called(ctx, schedulerID, updates)
	return args.Error(0)
}

// DeleteScheduler mocks the DeleteScheduler method
func (m *MockDB) DeleteScheduler(ctx context.Context, schedulerID string) error {
	args := m.Called(ctx, schedulerID)
	return args.Error(0)
}

// GetSchedulersReadyToRun mocks the GetSchedulersReadyToRun method
func (m *MockDB) GetSchedulersReadyToRun(ctx context.Context, limit int) ([]*db.Scheduler, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*db.Scheduler), args.Error(1)
}

// UpdateSchedulerNextRun mocks the UpdateSchedulerNextRun method
func (m *MockDB) UpdateSchedulerNextRun(ctx context.Context, schedulerID string, nextRun time.Time) error {
	args := m.Called(ctx, schedulerID, nextRun)
	return args.Error(0)
}

// GetDomainNameByID mocks the GetDomainNameByID method
func (m *MockDB) GetDomainNameByID(ctx context.Context, domainID int) (string, error) {
	args := m.Called(ctx, domainID)
	return args.String(0), args.Error(1)
}

// GetDomainNames mocks the GetDomainNames method
func (m *MockDB) GetDomainNames(ctx context.Context, domainIDs []int) (map[int]string, error) {
	args := m.Called(ctx, domainIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[int]string), args.Error(1)
}

// GetLastJobStartTimeForScheduler mocks the GetLastJobStartTimeForScheduler method
func (m *MockDB) GetLastJobStartTimeForScheduler(ctx context.Context, schedulerID string) (*time.Time, error) {
	args := m.Called(ctx, schedulerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*time.Time), args.Error(1)
}

// ListUserOrganisations mocks the ListUserOrganisations method
func (m *MockDB) ListUserOrganisations(userID string) ([]db.UserOrganisation, error) {
	args := m.Called(userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]db.UserOrganisation), args.Error(1)
}

// ValidateOrganisationMembership mocks the ValidateOrganisationMembership method
func (m *MockDB) ValidateOrganisationMembership(userID, organisationID string) (bool, error) {
	args := m.Called(userID, organisationID)
	return args.Bool(0), args.Error(1)
}

// SetActiveOrganisation mocks the SetActiveOrganisation method
func (m *MockDB) SetActiveOrganisation(userID, organisationID string) error {
	args := m.Called(userID, organisationID)
	return args.Error(0)
}

// GetEffectiveOrganisationID mocks the GetEffectiveOrganisationID method
func (m *MockDB) GetEffectiveOrganisationID(user *db.User) string {
	args := m.Called(user)
	return args.String(0)
}

// CreateOrganisation mocks the CreateOrganisation method
func (m *MockDB) CreateOrganisation(name string) (*db.Organisation, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.Organisation), args.Error(1)
}

// AddOrganisationMember mocks the AddOrganisationMember method
func (m *MockDB) AddOrganisationMember(userID, organisationID string) error {
	args := m.Called(userID, organisationID)
	return args.Error(0)
}

// Slack integration methods

func (m *MockDB) CreateSlackConnection(ctx context.Context, conn *db.SlackConnection) error {
	args := m.Called(ctx, conn)
	return args.Error(0)
}

func (m *MockDB) GetSlackConnection(ctx context.Context, connectionID string) (*db.SlackConnection, error) {
	args := m.Called(ctx, connectionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.SlackConnection), args.Error(1)
}

func (m *MockDB) ListSlackConnections(ctx context.Context, organisationID string) ([]*db.SlackConnection, error) {
	args := m.Called(ctx, organisationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*db.SlackConnection), args.Error(1)
}

func (m *MockDB) DeleteSlackConnection(ctx context.Context, connectionID, organisationID string) error {
	args := m.Called(ctx, connectionID, organisationID)
	return args.Error(0)
}

func (m *MockDB) CreateSlackUserLink(ctx context.Context, link *db.SlackUserLink) error {
	args := m.Called(ctx, link)
	return args.Error(0)
}

func (m *MockDB) GetSlackUserLink(ctx context.Context, userID, connectionID string) (*db.SlackUserLink, error) {
	args := m.Called(ctx, userID, connectionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.SlackUserLink), args.Error(1)
}

func (m *MockDB) UpdateSlackUserLinkNotifications(ctx context.Context, userID, connectionID string, dmNotifications bool) error {
	args := m.Called(ctx, userID, connectionID, dmNotifications)
	return args.Error(0)
}

func (m *MockDB) DeleteSlackUserLink(ctx context.Context, userID, connectionID string) error {
	args := m.Called(ctx, userID, connectionID)
	return args.Error(0)
}

func (m *MockDB) StoreSlackToken(ctx context.Context, connectionID, token string) error {
	args := m.Called(ctx, connectionID, token)
	return args.Error(0)
}

func (m *MockDB) GetSlackToken(ctx context.Context, connectionID string) (string, error) {
	args := m.Called(ctx, connectionID)
	return args.String(0), args.Error(1)
}
