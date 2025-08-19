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
func (m *MockDB) ListJobs(organisationID string, limit, offset int, status, dateRange string) ([]db.JobWithDomain, int, error) {
	args := m.Called(organisationID, limit, offset, status, dateRange)
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